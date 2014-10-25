#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import sys
from setproctitle import setproctitle
import signal
import Queue


def run_job(sema, child_q, manager_q, provider, **settings):
    aquired = False
    setproctitle("tunasync-{}".format(provider.name))

    def before_quit(*args):
        provider.terminate()
        if aquired:
            print("{} release semaphore".format(provider.name))
            sema.release()
        sys.exit(0)

    signal.signal(signal.SIGTERM, before_quit)

    max_retry = settings.get("max_retry", 1)
    while 1:
        try:
            sema.acquire(True)
        except:
            break
        aquired = True

        status = "unkown"
        try:
            for hook in provider.hooks:
                hook.before_job(name=provider.name)
        except Exception:
            import traceback
            traceback.print_exc()
            status = "fail"
        else:
            for retry in range(max_retry):
                print("start syncing {}, retry: {}".format(provider.name, retry))
                provider.run()

                status = "success"
                try:
                    provider.wait()
                except sh.ErrorReturnCode:
                    status = "fail"

                if status == "success":
                    break

        try:
            for hook in provider.hooks[::-1]:
                hook.after_job(name=provider.name, status=status)
        except Exception:
            import traceback
            traceback.print_exc()
            status = "fail"

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
