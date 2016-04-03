#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import os
import argparse

from tunasync import TUNASync


if __name__ == "__main__":
    here = os.path.abspath(os.path.dirname(__file__))

    parser = argparse.ArgumentParser(prog="tunasync")
    parser.add_argument("-c", "--config",
                        default="tunasync.ini", help="config file")
    parser.add_argument("--pidfile", default="/run/tunasync/tunasync.pid",
                        help="pidfile")

    args = parser.parse_args()

    with open(args.pidfile, 'w') as f:
        f.write("{}".format(os.getpid()))

    tunaSync = TUNASync()
    tunaSync.read_config(args.config)

    tunaSync.run_jobs()

# vim: ts=4 sw=4 sts=4 expandtab
