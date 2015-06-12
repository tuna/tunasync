#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import os
import sh
import shlex
from .hook import JobHook


class CmdExecHook(JobHook):
    POST_SYNC = "post_sync"
    PRE_SYNC = "pre_sync"

    def __init__(self, command, exec_at=POST_SYNC):
        self.command = shlex.split(command)
        if exec_at == self.POST_SYNC:
            self.before_job = self._keep_calm
            self.after_job = self._exec
        elif exec_at == self.PRE_SYNC:
            self.before_job = self._exec
            self.after_job = self._keep_calm

    def _keep_calm(self, ctx={}, **kwargs):
        pass

    def _exec(self, ctx={}, **kwargs):
        new_env = os.environ.copy()
        new_env["TUNASYNC_MIRROR_NAME"] = ctx["mirror_name"]
        new_env["TUNASYNC_WORKING_DIR"] = ctx["current_dir"]
        new_env["TUNASYNC_JOB_EXIT_STATUS"] = kwargs.get("status", "")

        _cmd = self.command[0]
        _args = [] if len(self.command) == 1 else self.command[1:]
        cmd = sh.Command(_cmd)
        cmd(*_args, _env=new_env)

# vim: ts=4 sw=4 sts=4 expandtab
