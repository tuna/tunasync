#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import os
from .hook import JobHook
from datetime import datetime


class LogLimitHook(JobHook):

    def __init__(self, limit=10):
        self.limit = limit

    def before_job(self, provider, ctx={}, *args, **kwargs):
        log_dir = provider.log_dir
        self.ensure_log_dir(log_dir)
        log_file = provider.log_file.format(
            date=datetime.now().strftime("%Y-%m-%d_%H-%M"))
        ctx['log_file'] = log_file
        if log_file == "/dev/null":
            return

        log_link = os.path.join(log_dir, "latest")
        ctx['log_link'] = log_link

        lfiles = [os.path.join(log_dir, lfile)
                  for lfile in os.listdir(log_dir)
                  if lfile.startswith(provider.name)]

        lfiles_set = set(lfiles)
        # sort to get the newest 10 files
        lfiles_ts = sorted(
            [(os.path.getmtime(lfile), lfile) for lfile in lfiles],
            key=lambda x: x[0],
            reverse=True)
        lfiles_keep = set([x[1] for x in lfiles_ts[:self.limit]])
        lfiles_rm = lfiles_set - lfiles_keep
        # remove old files
        for lfile in lfiles_rm:
            try:
                sh.rm(lfile)
            except:
                pass

        # create a soft link
        self.create_link(log_link, log_file)

    def after_job(self, status=None, ctx={}, *args, **kwargs):
        log_file = ctx.get('log_file', None)
        log_link = ctx.get('log_link', None)
        if log_file == "/dev/null":
            return
        if status == "fail":
            log_file_save = log_file + ".fail"
            try:
                sh.mv(log_file, log_file_save)
            except:
                pass
            self.create_link(log_link, log_file_save)

    def ensure_log_dir(self, log_dir):
        if not os.path.exists(log_dir):
            sh.mkdir("-p", log_dir)

    def create_link(self, log_link, log_file):
        if log_link == log_file:
            return
        if not (log_link and log_file):
            return

        if os.path.lexists(log_link):
            try:
                sh.rm(log_link)
            except:
                return
        try:
            sh.ln('-s', log_file, log_link)
        except:
            return


# vim: ts=4 sw=4 sts=4 expandtab
