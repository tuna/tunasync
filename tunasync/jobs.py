#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import sh
import sys
from setproctitle import setproctitle
import signal
import Queue
import traceback


def run_job(sema, child_q, manager_q, provider, **settings):
    aquired = False
    setproctitle("tunasync-{}".format(provider.name))

    def before_quit(*args):
        provider.terminate()
        if aquired:
            print("{} release semaphore".format(provider.name))
            sema.release()
        sys.exit(0)

    def sleep_wait(timeout):
        try:
            msg = child_q.get(timeout=timeout)
            if msg == "terminate":
                manager_q.put(("CONFIG_ACK", (provider.name, "QUIT")))
                return True
        except Queue.Empty:
            return False

    signal.signal(signal.SIGTERM, before_quit)

    if provider.delay > 0:
        if sleep_wait(provider.delay):
            return

    max_retry = settings.get("max_retry", 1)

    def _real_run(idx=0, stage="job_hook", ctx=None):
        """\
        4 stages:
            0 -> job_hook, 1 -> set_retry, 2 -> exec_hook, 3 -> exec
        """

        assert(ctx is not None)

        if stage == "exec":
            # exec_job
            try:
                provider.run(ctx=ctx)
                provider.wait()
            except sh.ErrorReturnCode:
                status = "fail"
            else:
                status = "success"
            return status

        elif stage == "set_retry":
            # enter stage 3 with retry
            for retry in range(max_retry):
                status = "syncing"
                manager_q.put(("UPDATE", (provider.name, status, ctx)))
                print("start syncing {}, retry: {}".format(provider.name, retry))
                status = _real_run(idx=0, stage="exec_hook", ctx=ctx)
                if status == "success":
                    break
            return status

        # job_hooks
        elif stage == "job_hook":
            if idx == len(provider.hooks):
                return _real_run(idx=idx, stage="set_retry", ctx=ctx)
            hook = provider.hooks[idx]
            hook_before, hook_after = hook.before_job, hook.after_job
            status = "pre-syncing"

        elif stage == "exec_hook":
            if idx == len(provider.hooks):
                return _real_run(idx=idx, stage="exec", ctx=ctx)
            hook = provider.hooks[idx]
            hook_before, hook_after = hook.before_exec, hook.after_exec
            status = "syncing"

        try:
            # print("%s run before_%s, %d" % (provider.name, stage, idx))
            hook_before(provider=provider, ctx=ctx)
            status = _real_run(idx=idx+1, stage=stage, ctx=ctx)
        except Exception:
            traceback.print_exc()
            status = "fail"
        finally:
            # print("%s run after_%s, %d" % (provider.name, stage, idx))
            # job may break when syncing
            if status != "success":
                status = "fail"
            try:
                hook_after(provider=provider, status=status, ctx=ctx)
            except Exception:
                traceback.print_exc()

        return status

    while 1:
        try:
            sema.acquire(True)
        except:
            break
        aquired = True

        ctx = {}   # put context info in it
        ctx['current_dir'] = provider.local_dir
        ctx['mirror_name'] = provider.name
        status = "pre-syncing"
        manager_q.put(("UPDATE", (provider.name, status, ctx)))

        try:
            status = _real_run(idx=0, stage="job_hook", ctx=ctx)
        except Exception:
            traceback.print_exc()
            status = "fail"
        finally:
            sema.release()
            aquired = False

        print("syncing {} finished, sleep {} minutes for the next turn".format(
            provider.name, provider.interval
        ))

        manager_q.put(("UPDATE", (provider.name, status, ctx)))

        if sleep_wait(timeout=provider.interval * 60):
            break


# vim: ts=4 sw=4 sts=4 expandtab
