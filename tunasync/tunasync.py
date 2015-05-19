#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import signal
import sys
import toml

from multiprocessing import Process, Semaphore, Queue
from . import jobs
from .hook import JobHook
from .mirror_config import MirrorConfig
from .status_manager import StatusManager
from .clt_server import run_control_server


class TUNASync(object):

    _instance = None
    _settings = None
    _inited = False

    def __new__(cls, *args, **kwargs):
        if not cls._instance:
            cls._instance = super(TUNASync, cls).__new__(cls, *args, **kwargs)

        return cls._instance

    def read_config(self, config_file):
        self._config_file = config_file
        with open(self._config_file) as f:
            self._settings = toml.loads(f.read())

        self._inited = True
        self._mirrors = {}
        self._providers = {}
        self.processes = {}
        self.semaphore = Semaphore(self._settings["global"]["concurrent"])
        self.channel = Queue()
        self._hooks = []

        self.mirror_root = self._settings["global"]["mirror_root"]

        self.use_btrfs = self._settings["global"]["use_btrfs"]
        self.btrfs_service_dir_tmpl = self._settings["btrfs"]["service_dir"]
        self.btrfs_working_dir_tmpl = self._settings["btrfs"]["working_dir"]
        self.btrfs_gc_dir_tmpl = self._settings["btrfs"]["gc_dir"]

        self.status_file = self._settings["global"]["status_file"]
        self.status_manager = StatusManager(self, self.status_file)

        self.ctrl_addr = self._settings["global"]["ctrl_addr"]
        self.ctrl_channel = Queue()
        p = Process(
            target=run_control_server,
            args=(self.ctrl_addr, self.channel, self.ctrl_channel),
        )
        p.start()
        self.processes["CTRL_SERVER"] = (self.ctrl_channel, p)

    def add_hook(self, h):
        assert isinstance(h, JobHook)
        self._hooks.append(h)

    def hooks(self):
        return self._hooks

    @property
    def mirrors(self):
        if self._mirrors:
            return self._mirrors

        for mirror_opt in self._settings["mirrors"]:
            name = mirror_opt["name"]
            self._mirrors[name] = \
                MirrorConfig(self, mirror_opt)

        return self._mirrors

    @property
    def providers(self):
        if self._providers:
            return self._providers

        for name, mirror in self.mirrors.iteritems():
            hooks = mirror.hooks() + self.hooks()
            provider = mirror.to_provider(hooks, no_delay=mirror.no_delay)
            self._providers[name] = provider

        return self._providers

    def run_jobs(self):
        for name in self.providers:
            self.run_provider(name)

        def sig_handler(*args):
            print("terminate subprocesses")
            for _, np in self.processes.iteritems():
                _, p = np
                p.terminate()
            print("Good Bye")
            sys.exit(0)

        signal.signal(signal.SIGINT, sig_handler)
        signal.signal(signal.SIGTERM, sig_handler)
        signal.signal(signal.SIGUSR1, self.reload_mirrors)
        signal.signal(signal.SIGUSR2, self.reload_mirrors_force)

        self.run_forever()

    def run_provider(self, name):
        if name not in self.providers:
            print("{} doesnot exist".format(name))
            return

        provider = self.providers[name]
        child_queue = Queue()
        p = Process(
            target=jobs.run_job,
            args=(self.semaphore, child_queue, self.channel, provider, ),
            kwargs={
                'max_retry': self._settings['global']['max_retry']}
        )
        p.start()
        provider.set_delay(0)  # clear delay after first start
        self.processes[name] = (child_queue, p)

    def reload_mirrors(self, signum, frame):
        try:
            return self._reload_mirrors(signum, frame, force=False)
        except Exception, e:
            print(e)

    def reload_mirrors_force(self, signum, frame):
        try:
            return self._reload_mirrors(signum, frame, force=True)
        except Exception, e:
            print(e)

    def _reload_mirrors(self, signum, frame, force=False):
        print("reload mirror configs, force restart: {}".format(force))

        with open(self._config_file) as f:
            self._settings = toml.loads(f.read())

        for mirror_opt in self._settings["mirrors"]:
            name = mirror_opt["name"]
            newMirCfg = MirrorConfig(self, mirror_opt)

            if name in self._mirrors:
                if newMirCfg.compare(self._mirrors[name]):
                    continue

            self._mirrors[name] = newMirCfg

            hooks = newMirCfg.hooks() + self.hooks()
            newProvider = newMirCfg.to_provider(hooks, no_delay=True)
            self._providers[name] = newProvider

            if name in self.processes:
                q, p = self.processes[name]

                if force:
                    p.terminate()
                    print("Terminated Job: {}".format(name))
                    self.run_provider(name)
                else:
                    q.put("terminate")
                    print("New configuration queued to {}".format(name))
            else:
                print("New mirror: {}".format(name))
                self.run_provider(name)

            self.status_manager.refresh_mirror(name)

    def run_forever(self):
        while 1:
            try:
                msg_hdr, msg_body = self.channel.get()
            except IOError:
                continue

            if msg_hdr == "UPDATE":
                mirror_name, status, ctx = msg_body
                try:
                    self.status_manager.update_status(
                        mirror_name, status, dict(ctx.items()))
                except Exception as e:
                    print(e)

            elif msg_hdr == "CONFIG_ACK":
                mirror_name, status = msg_body
                if status == "QUIT":
                    print("New configuration applied to {}".format(mirror_name))
                    self.run_provider(mirror_name)

            elif msg_hdr == "CMD":
                cmd, mirror_name, kwargs = msg_body
                if (mirror_name not in self.mirrors) and (mirror_name != "__ALL__"):
                    self.ctrl_channel.put("Invalid target")
                    continue
                res = self.handle_cmd(cmd, mirror_name, kwargs)
                self.ctrl_channel.put(res)

    def handle_cmd(self, cmd, mirror_name, kwargs):
        if cmd == "restart":
            _, p = self.processes[mirror_name]
            p.terminate()
            self.providers[mirror_name].set_delay(0)
            self.run_provider(mirror_name)
            res = "Restarted Job: {}".format(mirror_name)

        elif cmd == "stop":
            if mirror_name not in self.processes:
                res = "{} not running".format(mirror_name)
                return res

            _, p = self.processes.pop(mirror_name)
            p.terminate()
            res = "Stopped Job: {}".format(mirror_name)

        elif cmd == "start":
            if mirror_name in self.processes:
                res = "{} already running".format(mirror_name)
                return res

            self.run_provider(mirror_name)
            res = "Started Job: {}".format(mirror_name)

        elif cmd == "status":
            if mirror_name == "__ALL__":
                res = self.status_manager.list_status(_format=True)
            else:
                res = self.status_manager.get_status(mirror_name, _format=True)

        elif cmd == "log":
            job_ctx = self.status_manager.get_info(mirror_name, "ctx")
            n = kwargs.get("n", 0)
            if n == 0:
                res = job_ctx.get(
                    "log_link",
                    job_ctx.get("log_file", "/dev/null"),
                )
            else:
                import os
                log_file = job_ctx.get("log_file", None)
                if log_file is None:
                    return "/dev/null"

                log_dir = os.path.dirname(log_file)
                lfiles = [
                    os.path.join(log_dir, lfile)
                    for lfile in os.listdir(log_dir)
                    if lfile.startswith(mirror_name) and lfile != "latest"
                ]

                if len(lfiles) <= n:
                    res = "Only {} log files available".format(len(lfiles))
                    return res

                lfiles_set = set(lfiles)
                # sort to get the newest 10 files
                lfiles_ts = sorted(
                    [(os.path.getmtime(lfile), lfile) for lfile in lfiles],
                    key=lambda x: x[0],
                    reverse=True,
                )
                return lfiles_ts[n][1]

        else:
            res = "Invalid command"

        return res


# vim: ts=4 sw=4 sts=4 expandtab
