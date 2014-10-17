#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import time


def run_job(sema, provider):
    while 1:
        sema.acquire(True)
        print("start syncing {}".format(provider.name))

        for hook in provider.hooks:
            hook.before_job()

        provider.run()

        for hook in provider.hooks[::-1]:
            hook.after_job()

        sema.release()
        print("syncing {} finished, sleep {} minutes for the next turn".format(
            provider.name, provider.interval
        ))
        time.sleep(provider.interval * 60)


# vim: ts=4 sw=4 sts=4 expandtab
