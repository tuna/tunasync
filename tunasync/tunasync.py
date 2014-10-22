#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import ConfigParser
import os.path
import signal
import sys

from multiprocessing import Process, Semaphore, Queue
from . import jobs
from .mirror_provider import RsyncProvider, ShellProvider
from .btrfs_snapshot import BtrfsHook
from .hook import JobHook


class MirrorConfig(object):

    _valid_providers = set(("rsync", "debmirror", "shell", ))

    def __init__(self, parent, name, cfgParser, section):
        self._parent = parent
        self._cp = cfgParser
        self._sec = section

        self.name = name
        self.options = dict(self._cp.items(self._sec))
        self._validate()

    def _validate(self):
        provider = self.options.get("provider", None)
        assert provider in self._valid_providers

        if provider == "rsync":
            assert "upstream" in self.options
            if "use_ipv6" in self.options:
                self.options["use_ipv6"] = self._cp.getboolean(self._sec,
                                                               "use_ipv6")

        elif provider == "shell":
            assert "command" in self.options

        local_dir_tmpl = self.options.get(
            "local_dir", self._cp.get("global", "local_dir"))

        self.options["local_dir"] = local_dir_tmpl.format(
            mirror_root=self._cp.get("global", "mirror_root"),
            mirror_name=self.name,
        )

        self.options["interval"] = int(
            self.options.get("interval",
                             self._cp.getint("global", "interval"))
        )

        log_dir = self._cp.get("global", "log_dir")
        self.options["log_file"] = self.options.get(
            "log_file",
            os.path.join(log_dir, self.name, "{date}.log")
        )

        try:
            self.options["use_btrfs"] = self._cp.getboolean(
                self._sec, "use_btrfs")
        except ConfigParser.NoOptionError:
            self.options["use_btrfs"] = self._parent.use_btrfs

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
        self._settings = ConfigParser.ConfigParser()
        self._settings.read(config_file)

        self._inited = True
        self._mirrors = {}
        self._providers = {}
        self.processes = {}
        self.semaphore = Semaphore(self._settings.getint("global", "concurrent"))
        self.channel = Queue()
        self._hooks = []

        self.mirror_root = self._settings.get("global", "mirror_root")
        self.use_btrfs = self._settings.getboolean("global", "use_btrfs")
        self.btrfs_service_dir_tmpl = self._settings.get(
            "btrfs", "service_dir")
        self.btrfs_working_dir_tmpl = self._settings.get(
            "btrfs", "working_dir")
        self.btrfs_gc_dir_tmpl = self._settings.get(
            "btrfs", "gc_dir")

    def add_hook(self, h):
        assert isinstance(h, JobHook)
        self._hooks.append(h)

    def hooks(self):
        return self._hooks

    @property
    def mirrors(self):
        if self._mirrors:
            return self._mirrors

        for section in filter(lambda s: s.startswith("mirror:"),
                              self._settings.sections()):

            _, name = section.split(":")
            self._mirrors[name] = \
                MirrorConfig(self, name, self._settings, section)
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
            args=(self.semaphore, child_queue, self.channel, provider, )
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
        self._settings.read(self._config_file)

        for section in filter(lambda s: s.startswith("mirror:"),
                              self._settings.sections()):

            _, name = section.split(":")
            newMirCfg = MirrorConfig(self, name, self._settings, section)

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

    # def config(self, option):
    #     if self._settings is None:
    #         raise TUNASyncException("Config not inited")
    #


# vim: ts=4 sw=4 sts=4 expandtab
