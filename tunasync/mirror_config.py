#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import os
from .mirror_provider import RsyncProvider, ShellProvider
from .btrfs_snapshot import BtrfsHook
from .loglimit import LogLimitHook


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

        log_dir = self.options.get(
            "log_dir", self._popt["global"]["log_dir"])
        if "log_file" not in self.options:
            self.options["log_file"] = os.path.join(
                log_dir, self.name, self.name + "_{date}.log")

        self.log_dir = os.path.dirname(self.log_file)

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
                name=self.name,
                upstream_url=self.upstream,
                local_dir=self.local_dir,
                log_dir=self.log_dir,
                useIPv6=self.use_ipv6,
                password=self.password,
                exclude_file=self.exclude_file,
                log_file=self.log_file,
                interval=self.interval,
                hooks=hooks,
            )
        elif self.options["provider"] == "shell":
            provider = ShellProvider(
                name=self.name,
                command=self.command,
                upstream_url=self.upstream,
                local_dir=self.local_dir,
                log_dir=self.log_dir,
                log_file=self.log_file,
                interval=self.interval,
                hooks=hooks
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

        hooks.append(LogLimitHook())
        return hooks

# vim: ts=4 sw=4 sts=4 expandtab
