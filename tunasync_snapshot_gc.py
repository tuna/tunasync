#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import re
import sh
import os
import ConfigParser
import argparse

if __name__ == "__main__":
    parser = argparse.ArgumentParser(prog="tunasync_snapshot_gc")
    parser.add_argument("--max-level", type=int, default=2, help="max walk level to find garbage snapshots")
    parser.add_argument("--pattern", default=r"^_gc_\d+", help="pattern to match garbage snapshots")
    parser.add_argument("-c", "--config", help="tunasync config file")

    args = parser.parse_args()

    pattern = re.compile(args.pattern)

    def walk(_dir, level=1):
        if level > 2:
            return

        for fname in os.listdir(_dir):
            abs_fname = os.path.join(_dir, fname)
            if os.path.isdir(abs_fname):
                if pattern.match(fname):
                    print("GC: {}".format(abs_fname))
                    try:
                        ret = sh.btrfs("subvolume", "delete", abs_fname)
                    except sh.ErrorReturnCode:
                        print("Error: {}".format(ret.stderr))
                else:
                    walk(abs_fname, level+1)

    settings = ConfigParser.ConfigParser()
    settings.read(args.config)
    mirror_root = settings.get("global", "mirror_root")

    walk(mirror_root)

# vim: ts=4 sw=4 sts=4 expandtab
