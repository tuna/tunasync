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
            _mirrors = self.list_status()
            print("Updated status file, {}:{}".format(name, status))
            json.dump(_mirrors, f)

    def list_status(self, _format=False):
        _mirrors = sorted(
            [m for _, m in self.mirrors.items()],
            key=lambda x: x['name']
        )
        if not _format:
            return _mirrors

        name_len = max([len(_m['name']) for _m in _mirrors])
        update_len = max([len(_m['last_update']) for _m in _mirrors])
        status_len = max([len(_m['status']) for _m in _mirrors])
        heading = '  '.join([
            'name'.ljust(name_len),
            'last update'.ljust(update_len),
            'status'.ljust(status_len)
        ])
        line = '  '.join(['-'*name_len, '-'*update_len, '-'*status_len])
        tabular = '\n'.join(
            [
                '  '.join(
                    (_m['name'].ljust(name_len),
                     _m['last_update'].ljust(update_len),
                     _m['status'].ljust(status_len))
                ) for _m in _mirrors
            ]
        )
        return '\n'.join((heading, line, tabular))

    def get_status(self, name, _format=False):
        if name not in self.mirrors:
            return None

        mir = self.mirrors[name]
        if not _format:
            return mir

        tmpl = "{name}  last_update: {last_update}  status: {status}"
        return tmpl.format(**mir)


# vim: ts=4 sw=4 sts=4 expandtab
