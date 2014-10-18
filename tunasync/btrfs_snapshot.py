#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import os
from .hook import JobHook


class BtrfsVolumeError(Exception):
    pass


class BtrfsHook(JobHook):

    def __init__(self, service_dir, working_dir, tmp_dir):
        self.service_dir = service_dir
        self.working_dir = working_dir
        self.tmp_dir = tmp_dir

    def before_job(self):
        self._create_working_snapshot()

    def after_job(self):
        self._commit_changes()

    def _ensure_subvolume(self):
        # print(self.service_dir)
        try:
            ret = sh.btrfs("subvolume", "show", self.service_dir)
        except Exception, e:
            print(e)
            raise BtrfsVolumeError("Invalid subvolume")

        if ret.stderr != '':
            raise BtrfsVolumeError("Invalid subvolume")

    def _create_working_snapshot(self):
        self._ensure_subvolume()
        if os.path.exists(self.working_dir):
            print("Warning: working dir existed, are you sure no rsync job is running?")
        else:
            # print("btrfs subvolume snapshot {} {}".format(self.service_dir, self.working_dir))
            sh.btrfs("subvolume", "snapshot", self.service_dir, self.working_dir)

    def _commit_changes(self):
        self._ensure_subvolume()
        self._ensure_subvolume()
        out = sh.mv(self.service_dir, self.tmp_dir)
        assert out.exit_code == 0 and out.stderr == ""
        out = sh.mv(self.working_dir, self.service_dir)
        assert out.exit_code == 0 and out.stderr == ""
        # print("btrfs subvolume delete {}".format(self.tmp_dir))
        sh.sleep(3)
        out = sh.btrfs("subvolume", "delete", self.tmp_dir)
        assert out.exit_code == 0 and out.stderr == ""

# vim: ts=4 sw=4 sts=4 expandtab
