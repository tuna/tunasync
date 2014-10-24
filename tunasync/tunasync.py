#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import os.path
import signal
import sys
import toml

from multiprocessing import Process, Semaphore, Queue
from . import jobs
from .mirror_provider import RsyncProvider, ShellProvider
from .btrfs_snapshot import BtrfsHook
from .hook import JobHook


class MirrorConfig(object):

    _valid_providers = set(("rsync", "debmirror", "shell", ))

    def __init__(self, parent, options):
        self._parent = parent
        self._popt = self._parent._settings
        self.options = dict(options.items())  # copy
        self._validate()

    def _validate(self):
        provider = self.options.get("provider", None)
        assert provider in self._valid_providers

        if provider == "rsync":
            assert "upstream" in self.options

        elif provider == "shell":
            assert "command" in self.options

        local_dir_tmpl = self.options.get(
            "local_dir", self._popt["global"]["local_dir"])

        self.options["local_dir"] = local_dir_tmpl.format(
            mirror_root=self._popt["global"]["mirror_root"],
            mirror_name=self.name,
        )

        if "interval" not in self.options:
            self.options["interval"] = self._popt["global"]["interval"]

        assert isinstance(self.options["interval"], int)

        log_dir = self._popt["global"]["log_dir"]
        if "log_file" not in self.options:
            self.options["log_file"] = os.path.join(
                log_dir, self.name, "{date}.log")

        if "use_btrfs" not in self.options:
            self.options["use_btrfs"] = self._parent.use_btrfs
        assert self.options["use_btrfs"] in (True, False)

    def __getattr__(self, key):
        if key in self.__dict__:
            return self.__dict__[key]
        else:
            return self.__dict__["options"].get(key, None)

    def to_provider(self, hooks=[]):
        if self.provider == "rsync":
            provider = RsyncProvider(
                self.name,
                self.upstream,
                self.local_dir,
                self.use_ipv6,
                self.exclude_file,
                self.log_file,
                self.interval,
                hooks,
            )
        elif self.options["provider"] == "shell":
            provider = ShellProvider(
                self.name,
                self.command,
                self.local_dir,
                self.log_file,
                self.interval,
                hooks
            )

        return provider

    def compare(self, other):
        assert self.name == other.name

        for key, val in self.options.iteritems():
            if other.options.get(key, None) != val:
                return False

        return True

    def hooks(self):
        hooks = []
        parent = self._parent
        if self.options["use_btrfs"]:
            working_dir = parent.btrfs_working_dir_tmpl.format(
                mirror_root=parent.mirror_root,
                mirror_name=self.name
            )
            service_dir = parent.btrfs_service_dir_tmpl.format(
                mirror_root=parent.mirror_root,
                mirror_name=self.name
            )
            gc_dir = parent.btrfs_gc_dir_tmpl.format(
                mirror_root=parent.mirror_root,
                mirror_name=self.name
            )
            hooks.append(BtrfsHook(service_dir, working_dir, gc_dir))

        return hooks


class TUNASync(object):

    _instance = None
    _settings = None
    _inited = False

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            cls._instance = super(TUNASync, cls).__new__(cls, *args, **kwargs)

        return cls._instance

    def read_config(self, config_file):
        self._config_file = config_file
        with open(self._config_file) as f:
            self._settings = toml.loads(f.read())

        self._inited = True
        self._mirrors = {}
        self._providers = {}
        self.processes = {}
        self.semaphore = Semaphore(self._settings["global"]["concurrent"])
        self.channel = Queue()
        self._hooks = []

        self.mirror_root = self._settings["global"]["mirror_root"]

        self.use_btrfs = self._settings["global"]["use_btrfs"]
        self.btrfs_service_dir_tmpl = self._settings["btrfs"]["service_dir"]
        self.btrfs_working_dir_tmpl = self._settings["btrfs"]["working_dir"]
        self.btrfs_gc_dir_tmpl = self._settings["btrfs"]["gc_dir"]

    def add_hook(self, h):
        assert isinstance(h, JobHook)
        self._hooks.append(h)

    def hooks(self):
        return self._hooks

    @property
    def mirrors(self):
        if self._mirrors:
            return self._mirrors

        for mirror_opt in self._settings["mirrors"]:
            name = mirror_opt["name"]
            self._mirrors[name] = \
                MirrorConfig(self, mirror_opt)

        return self._mirrors

    @property
    def providers(self):
        if self._providers:
            return self._providers

        for name, mirror in self.mirrors.iteritems():
            hooks = mirror.hooks() + self.hooks()
            provider = mirror.to_provider(hooks)
            self._providers[name] = provider

        return self._providers

    def run_jobs(self):
        for name in self.providers:
            self.run_provider(name)

        def sig_handler(*args):
            print("terminate subprocesses")
            for _, np in self.processes.iteritems():
                _, p = np
                p.terminate()
            print("Good Bye")
            sys.exit(0)

        signal.signal(signal.SIGINT, sig_handler)
        signal.signal(signal.SIGTERM, sig_handler)
        signal.signal(signal.SIGUSR1, self.reload_mirrors)
        signal.signal(signal.SIGUSR2, self.reload_mirrors_force)

        while 1:
            try:
                name, status = self.channel.get()
            except IOError:
                continue

            if status == "QUIT":
                print("New configuration applied to {}".format(name))
                self.run_provider(name)

    def run_provider(self, name):
        if name not in self.providers:
            print("{} doesnot exist".format(name))
            return

        provider = self.providers[name]
        child_queue = Queue()
        p = Process(
            target=jobs.run_job,
            args=(self.semaphore, child_queue, self.channel, provider, ),
            kwargs={'max_retry': self._settings['global']['max_retry']}
        )
        p.start()
        self.processes[name] = (child_queue, p)

    def reload_mirrors(self, signum, frame):
        try:
            return self._reload_mirrors(signum, frame, force=False)
        except Exception, e:
            print(e)

    def reload_mirrors_force(self, signum, frame):
        try:
            return self._reload_mirrors(signum, frame, force=True)
        except Exception, e:
            print(e)

    def _reload_mirrors(self, signum, frame, force=False):
        print("reload mirror configs, force restart: {}".format(force))

        with open(self._config_file) as f:
            self._settings = toml.loads(f.read())

        for mirror_opt in self._settings["mirrors"]:
            name = mirror_opt["name"]
            newMirCfg = MirrorConfig(self, mirror_opt)

            if name in self._mirrors:
                if newMirCfg.compare(self._mirrors[name]):
                    continue

            self._mirrors[name] = newMirCfg

            hooks = newMirCfg.hooks() + self.hooks()
            newProvider = newMirCfg.to_provider(hooks)
            self._providers[name] = newProvider

            if name in self.processes:
                q, p = self.processes[name]

                if force:
                    p.terminate()
                    print("Terminated Job: {}".format(name))
                    self.run_provider(name)
                else:
                    q.put("terminate")
                    print("New configuration queued to {}".format(name))
            else:
                print("New mirror: {}".format(name))
                self.run_provider(name)


# vim: ts=4 sw=4 sts=4 expandtab
