#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import os
import shlex
from datetime import datetime


class MirrorProvider(object):
    '''
    Mirror method class, can be `rsync', `debmirror', etc.
    '''

    def __init__(self, name, local_dir, log_dir, log_file="/dev/null",
                 interval=120, hooks=[]):
        self.name = name
        self.local_dir = local_dir
        self.log_file = log_file
        self.log_dir = log_dir
        self.interval = interval
        self.hooks = hooks
        self.p = None
        self.delay = 0

    # deprecated
    def ensure_log_dir(self):
        log_dir = os.path.dirname(self.log_file)
        if not os.path.exists(log_dir):
            sh.mkdir("-p", log_dir)

    def get_log_file(self, ctx={}):
        if 'log_file' in ctx:
            log_file = ctx['log_file']
        else:
            now = datetime.now().strftime("%Y-%m-%d_%H")
            log_file = self.log_file.format(date=now)
            ctx['log_file'] = log_file
        return log_file

    def set_delay(self, sec):
        ''' Set start delay '''
        self.delay = sec

    def run(self, ctx={}):
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

    _default_options = ['-aHvh', '--no-o', '--no-g', '--stats',
                        '--exclude', '.~tmp~/',
                        '--delete', '--delete-after', '--delay-updates',
                        '--safe-links', '--timeout=120', '--contimeout=120']

    def __init__(self, name, upstream_url, local_dir, log_dir,
                 useIPv6=True, password=None, exclude_file=None,
                 log_file="/dev/null", interval=120, hooks=[]):
        super(RsyncProvider, self).__init__(name, local_dir, log_dir, log_file,
                                            interval, hooks)

        self.upstream_url = upstream_url
        self.useIPv6 = useIPv6
        self.exclude_file = exclude_file
        self.password = password

    @property
    def options(self):

        _options = [o for o in self._default_options]  # copy

        if self.useIPv6:
            _options.append("-6")

        if self.exclude_file:
            _options.append("--exclude-from")
            _options.append(self.exclude_file)

        return _options

    def run(self, ctx={}):
        _args = self.options
        _args.append(self.upstream_url)

        working_dir = ctx.get("current_dir", self.local_dir)
        _args.append(working_dir)

        log_file = self.get_log_file(ctx)
        new_env = os.environ.copy()
        if self.password is not None:
            new_env["RSYNC_PASSWORD"] = self.password

        self.p = sh.rsync(*_args, _env=new_env, _out=log_file,
                          _err_to_out=True, _out_bufsize=1, _bg=True)


class TwoStageRsyncProvider(RsyncProvider):

    _stage1_options = ['-aHvh', '--no-o', '--no-g',
                       '--exclude', '.~tmp~/',
                       '--safe-links', '--timeout=120', '--contimeout=120']

    _stage2_options = ['-aHvh', '--no-o', '--no-g', '--stats',
                       '--exclude', '.~tmp~/',
                       '--delete', '--delete-after', '--delay-updates',
                       '--safe-links', '--timeout=120', '--contimeout=120']

    _stage1_profiles = {
        "debian": [
            'Packages*', 'Sources*', 'Release*',
            'InRelease', 'i18n/*', 'ls-lR*',
        ]
    }

    def set_stage1_profile(self, profile):
        if profile not in self._stage1_profiles:
            raise Exception("Profile Undefined: %s, %s" % (profile, self.name))

        self._stage1_excludes = self._stage1_profiles[profile]

    def options(self, stage):
        _default_options = self._stage1_options \
            if stage == 1 else self._stage2_options
        _options = [o for o in _default_options]  # copy

        if stage == 1:
            for _exc in self._stage1_excludes:
                _options.append("--exclude")
                _options.append(_exc)

        if self.useIPv6:
            _options.append("-6")

        if self.exclude_file:
            _options.append("--exclude-from")
            _options.append(self.exclude_file)

        return _options

    def run(self, ctx={}):
        working_dir = ctx.get("current_dir", self.local_dir)
        log_file = self.get_log_file(ctx)
        new_env = os.environ.copy()
        if self.password is not None:
            new_env["RSYNC_PASSWORD"] = self.password

        with open(log_file, 'w', buffering=1) as f:
            def log_output(line):
                f.write(line)

            for stage in (1, 2):

                _args = self.options(stage)
                _args.append(self.upstream_url)
                _args.append(working_dir)
                f.write("==== Stage {} Begins ====\n\n".format(stage))

                self.p = sh.rsync(
                    *_args, _env=new_env, _out=log_output,
                    _err_to_out=True, _out_bufsize=1, _bg=False
                )
                self.p.wait()


class ShellProvider(MirrorProvider):

    def __init__(self, name, command, upstream_url, local_dir, log_dir,
                 log_file="/dev/null", log_stdout=True, interval=120, hooks=[]):

        super(ShellProvider, self).__init__(name, local_dir, log_dir, log_file,
                                            interval, hooks)
        self.upstream_url = str(upstream_url)
        self.command = shlex.split(command)
        self.log_stdout = log_stdout

    def run(self, ctx={}):

        log_file = self.get_log_file(ctx)

        new_env = os.environ.copy()
        new_env["TUNASYNC_MIRROR_NAME"] = self.name
        new_env["TUNASYNC_LOCAL_DIR"] = self.local_dir
        new_env["TUNASYNC_WORKING_DIR"] = ctx.get("current_dir", self.local_dir)
        new_env["TUNASYNC_UPSTREAM_URL"] = self.upstream_url
        new_env["TUNASYNC_LOG_FILE"] = log_file

        _cmd = self.command[0]
        _args = [] if len(self.command) == 1 else self.command[1:]

        cmd = sh.Command(_cmd)

        if self.log_stdout:
            self.p = cmd(*_args, _env=new_env, _out=log_file,
                         _err_to_out=True, _out_bufsize=1, _bg=True)
        else:
            self.p = cmd(*_args, _env=new_env, _out='/dev/null',
                         _err='/dev/null', _out_bufsize=1, _bg=True)


# vim: ts=4 sw=4 sts=4 expandtab
