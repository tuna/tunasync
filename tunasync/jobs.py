#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sys
import time
import signal


def run_job(sema, child_q, manager_q, provider):
    aquired = False

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
            hook.before_job()

        provider.run()
        provider.wait()

        for hook in provider.hooks[::-1]:
            hook.after_job()

        sema.release()
        aquired = False
        try:
            msg = child_q.get(timeout=1)
            if msg == "terminate":
                manager_q.put((provider.name, "QUIT"))
                break
        except:
            pass

        print("syncing {} finished, sleep {} minutes for the next turn".format(
            provider.name, provider.interval
        ))
        time.sleep(provider.interval * 60)


# vim: ts=4 sw=4 sts=4 expandtab
