#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import sys
from setproctitle import setproctitle
import signal
import Queue


def run_job(sema, child_q, manager_q, provider):
    aquired = False
    setproctitle("tunasync-{}".format(provider.name))

    def before_quit(*args):
        provider.terminate()
        if aquired:
            print("{} release semaphore".format(provider.name))
            sema.release()
        sys.exit(0)

    signal.signal(signal.SIGTERM, before_quit)

    while 1:
        try:
            sema.acquire(True)
        except:
            break
        aquired = True
        print("start syncing {}".format(provider.name))

        for hook in provider.hooks:
            hook.before_job(name=provider.name)

        provider.run()

        status = "success"
        try:
            provider.wait()
        except sh.ErrorReturnCode:
            status = "fail"

        for hook in provider.hooks[::-1]:
            try:
                hook.after_job(name=provider.name, status=status)
            except Exception:
                import traceback
                traceback.print_exc()

        sema.release()
        aquired = False

        print("syncing {} finished, sleep {} minutes for the next turn".format(
            provider.name, provider.interval
        ))

        try:
            msg = child_q.get(timeout=provider.interval * 60)
            if msg == "terminate":
                manager_q.put((provider.name, "QUIT"))
                break
        except Queue.Empty:
            pass


# vim: ts=4 sw=4 sts=4 expandtab
