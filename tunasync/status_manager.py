#!/usr/bin/env python2
# -*- coding:utf-8 -*-
import json
from datetime import datetime


class StatusManager(object):

    def __init__(self, parent, dbfile):
        self.parent = parent
        self.dbfile = dbfile
        self.init_mirrors()

    def init_mirrors(self):
        mirrors = {}
        try:
            with open(self.dbfile) as f:
                _mirrors = json.load(f)
                for m in _mirrors:
                    mirrors[m["name"]] = m
        except:
            for name, _ in self.parent.mirrors.iteritems():
                mirrors[name] = {
                    'name': name,
                    'last_update': '-',
                    'status': 'unknown',
                }
        self.mirrors = mirrors

    def update_status(self, name, status):

        _m = self.mirrors.get(name, {
            'name': name,
            'last_update': '-',
            'status': '-',
        })

        if status in ("syncing", "fail"):
            update_time = _m["last_update"]
        elif status == "success":
            update_time = datetime.now().strftime("%Y-%m-%d %H:%M:%S")
        else:
            print("Invalid status: {}, from {}".format(status, name))

        self.mirrors[name] = {
            'name': name,
            'last_update': update_time,
            'status': status,
        }

        with open(self.dbfile, 'wb') as f:
            _mirrors = sorted(
                [m for _, m in self.mirrors.items()],
                key=lambda x: x['name']
            )

            print("Updated status file, {}:{}".format(name, status))
            json.dump(_mirrors, f)


# vim: ts=4 sw=4 sts=4 expandtab
