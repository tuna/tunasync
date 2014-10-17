#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import ConfigParser
import os.path
import signal

from multiprocessing import Process, Semaphore
from . import jobs
from .mirror_provider import RsyncProvider, ShellProvider
from .btrfs_snapshot import BtrfsHook


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


class TUNASync(object):

    _instance = None
    _settings = None
    _inited = False

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            cls._instance = super(TUNASync, cls).__new__(cls, *args, **kwargs)

        return cls._instance

    def read_config(self, config_file):
        self._settings = ConfigParser.ConfigParser()
        self._settings.read(config_file)

        self._inited = True
        self._mirrors = []
        self._providers = []
        self.processes = []
        self.semaphore = Semaphore(self._settings.getint("global", "concurrent"))

        self.mirror_root = self._settings.get("global", "mirror_root")
        self.use_btrfs = self._settings.getboolean("global", "use_btrfs")
        self.btrfs_service_dir_tmpl = self._settings.get(
            "btrfs", "service_dir")
        self.btrfs_working_dir_tmpl = self._settings.get(
            "btrfs", "working_dir")
        self.btrfs_tmp_dir_tmpl = self._settings.get(
            "btrfs", "tmp_dir")

    @property
    def mirrors(self):
        if self._mirrors:
            return self._mirrors

        for section in filter(lambda s: s.startswith("mirror:"),
                              self._settings.sections()):

            _, name = section.split(":")
            self._mirrors.append(
                MirrorConfig(self, name, self._settings, section))
        return self._mirrors

    @property
    def providers(self):
        if self._providers:
            return self._providers

        for mirror in self.mirrors:
            hooks = []
            if mirror.options["use_btrfs"]:
                working_dir = self.btrfs_working_dir_tmpl.format(
                    mirror_root=self.mirror_root,
                    mirror_name=mirror.name
                )
                service_dir = self.btrfs_service_dir_tmpl.format(
                    mirror_root=self.mirror_root,
                    mirror_name=mirror.name
                )
                tmp_dir = self.btrfs_tmp_dir_tmpl.format(
                    mirror_root=self.mirror_root,
                    mirror_name=mirror.name
                )
                hooks.append(BtrfsHook(service_dir, working_dir, tmp_dir))

            if mirror.options["provider"] == "rsync":
                self._providers.append(
                    RsyncProvider(
                        mirror.name,
                        mirror.options["upstream"],
                        mirror.options["local_dir"],
                        mirror.options["use_ipv6"],
                        mirror.options.get("exclude_file", None),
                        mirror.options["log_file"],
                        mirror.options["interval"],
                        hooks,
                    )
                )
            elif mirror.options["provider"] == "shell":
                self._providers.append(
                    ShellProvider(
                        mirror.name,
                        mirror.options["command"],
                        mirror.options["local_dir"],
                        mirror.options["log_file"],
                        mirror.options["interval"],
                        hooks,
                    )
                )

        return self._providers

    def run_jobs(self):
        for provider in self.providers:
            p = Process(target=jobs.run_job, args=(self.semaphore, provider, ))
            p.start()
            self.processes.append(p)

        def sig_handler(*args):
            print("terminate subprocesses")
            for p in self.processes:
                p.terminate()
            print("Good Bye")

        signal.signal(signal.SIGINT, sig_handler)
        signal.signal(signal.SIGTERM, sig_handler)

    # def config(self, option):
    #     if self._settings is None:
    #         raise TUNASyncException("Config not inited")
    #


# vim: ts=4 sw=4 sts=4 expandtab
