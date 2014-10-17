#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import ConfigParser
import os.path
import signal

from multiprocessing import Process, Semaphore
from . import jobs
from .mirror_provider import RsyncProvider, ShellProvider


class MirrorConfig(object):

    _valid_providers = set(("rsync", "debmirror", "shell", ))

    def __init__(self, name, cfgParser, section):
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

        if "local_dir" not in self.options:
            self.options["local_dir"] = os.path.join(
                self._cp.get("global", "local_dir"),
                self.name)

        self.options["interval"] = int(
            self.options.get("interval",
                             self._cp.getint("global", "interval"))
        )

        log_dir = self._cp.get("global", "log_dir")
        self.options["log_file"] = self.options.get(
            "log_file",
            os.path.join(log_dir, self.name, "{date}.log")
        )


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

    @property
    def mirrors(self):
        if self._mirrors:
            return self._mirrors

        for section in filter(lambda s: s.startswith("mirror:"),
                              self._settings.sections()):

            _, name = section.split(":")
            self._mirrors.append(
                MirrorConfig(name, self._settings, section))
        return self._mirrors

    @property
    def providers(self):
        if self._providers:
            return self._providers

        for mirror in self.mirrors:
            if mirror.options["provider"] == "rsync":
                self._providers.append(
                    RsyncProvider(
                        mirror.name,
                        mirror.options["upstream"],
                        mirror.options["local_dir"],
                        mirror.options["use_ipv6"],
                        mirror.options.get("exclude_file", None),
                        mirror.options["log_file"],
                        mirror.options["interval"]
                    )
                )
            elif mirror.options["provider"] == "shell":
                self._providers.append(
                    ShellProvider(
                        mirror.name,
                        mirror.options["command"],
                        mirror.options["local_dir"],
                        mirror.options["log_file"],
                        mirror.options["interval"]
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
