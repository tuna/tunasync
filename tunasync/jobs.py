#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sys
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

        print("syncing {} finished, sleep {} minutes for the next turn".format(
            provider.name, provider.interval
        ))

        try:
            msg = child_q.get(timeout=provider.interval * 60)
            if msg == "terminate":
                manager_q.put((provider.name, "QUIT"))
                break
        except:
            pass


# vim: ts=4 sw=4 sts=4 expandtab
