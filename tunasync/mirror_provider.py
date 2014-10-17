#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import os
from datetime import datetime


class MirrorProvider(object):
    '''
    Mirror method class, can be `rsync', `debmirror', etc.
    '''

    def run(self):
        raise NotImplementedError("run method should be implemented")


class RsyncProvider(MirrorProvider):

    _default_options = "-av --delete-after"

    def __init__(self, name, upstream_url, local_dir, useIPv6=True,
                 exclude_file=None, log_file="/dev/null", interval=120):

        self.name = name
        self.upstream_url = upstream_url
        self.local_dir = local_dir
        self.useIPv6 = useIPv6
        self.exclude_file = exclude_file
        self.log_file = log_file
        self.interval = interval

    @property
    def options(self):

        _options = self._default_options.split()

        if self.useIPv6:
            _options.append("-6")
        else:
            _options.append("-4")

        if self.exclude_file:
            _options.append("--exclude-from")
            _options.append(self.exclude_file)

        return _options

    def run(self):
        _args = self.options
        _args.append(self.upstream_url)
        _args.append(self.local_dir)
        now = datetime.now().strftime("%Y-%m-%d_%H")
        log_file = self.log_file.format(date=now)

        sh.rsync(*_args, _out=log_file, _err=log_file)


class ShellProvider(MirrorProvider):

    def __init__(self, name, command, local_dir,
                 log_file="/dev/null", interval=120):
        self.name = name
        self.command = command.split()
        self.local_dir = local_dir
        self.log_file = log_file
        self.interval = interval

    def run(self):
        now = datetime.now().strftime("%Y-%m-%d_%H")
        log_file = self.log_file.format(date=now)

        new_env = os.environ.copy()
        new_env["TUNASYNC_LOCAL_DIR"] = self.local_dir
        new_env["TUNASYNC_LOG_FILE"] = log_file

        _cmd = self.command[0]
        _args = [] if len(self.command) == 1 else self.command[1:]

        cmd = sh.Command(_cmd)
        cmd(*_args, _env=new_env, _out=log_file, _err=log_file)


# vim: ts=4 sw=4 sts=4 expandtab
