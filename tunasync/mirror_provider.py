#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import os
from datetime import datetime


class MirrorProvider(object):
    '''
    Mirror method class, can be `rsync', `debmirror', etc.
    '''

    def __init__(self, name, local_dir, log_file="/dev/null",
                 interval=120, hooks=[]):
        self.name = name
        self.local_dir = local_dir
        self.log_file = log_file
        self.interval = interval
        self.hooks = hooks
        self.p = None

    def run(self):
        raise NotImplementedError("run method should be implemented")

    def terminate(self):
        if self.p is not None:
            self.p.process.terminate()
            print("{} terminated".format(self.name))
            self.p = None

    def wait(self):
        if self.p is not None:
            self.p.wait()
            self.p = None


class RsyncProvider(MirrorProvider):

    _default_options = "-av --delete-after"

    def __init__(self, name, upstream_url, local_dir, useIPv6=True,
                 exclude_file=None, log_file="/dev/null", interval=120,
                 hooks=[]):
        super(RsyncProvider, self).__init__(name, local_dir, log_file,
                                            interval, hooks)

        self.upstream_url = upstream_url
        self.useIPv6 = useIPv6
        self.exclude_file = exclude_file

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

        self.p = sh.rsync(*_args, _out=log_file, _err=log_file,
                          _out_bufsize=1, _bg=True)


class ShellProvider(MirrorProvider):

    def __init__(self, name, command, local_dir,
                 log_file="/dev/null", interval=120, hooks=[]):

        super(ShellProvider, self).__init__(name, local_dir, log_file,
                                            interval, hooks)
        self.command = command.split()

    def run(self):

        now = datetime.now().strftime("%Y-%m-%d_%H")
        log_file = self.log_file.format(date=now)

        new_env = os.environ.copy()
        new_env["TUNASYNC_MIRROR_NAME"] = self.name
        new_env["TUNASYNC_LOCAL_DIR"] = self.local_dir
        new_env["TUNASYNC_LOG_FILE"] = log_file

        _cmd = self.command[0]
        _args = [] if len(self.command) == 1 else self.command[1:]

        cmd = sh.Command(_cmd)
        self.p = cmd(*_args, _env=new_env, _out=log_file,
                     _err=log_file, _out_bufsize=1, _bg=True)


# vim: ts=4 sw=4 sts=4 expandtab
