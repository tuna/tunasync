#!/usr/bin/env python

ARCH_EXCLUDE = ['armel', 'arm64', 'alpha', 'hurd-i386', 'ia64', 'kfreebsd-amd64', 'kfreebsd-i386', 'mips', 'powerpc', 'ppc64el',  's390', 's390x', 'sparc']

CONTENT_EXCLUDE = ['binary-{arch}', 'installer-{arch}', 'Contents-{arch}.gz', 'Contents-udeb-{arch}.gz', 'Contents-{arch}.diff', 'arch-{arch}.files', 'arch-{arch}.list.gz', '*_{arch}.deb', '*_{arch}.udeb', '*_{arch}.changes']

with open("debian-exclude.txt", 'wb') as f:
	for arch in ARCH_EXCLUDE:
		for content in CONTENT_EXCLUDE:
			f.write(content.format(arch=arch))
			f.write('\n')
