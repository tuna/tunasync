#!/usr/bin/env python

ARCH_EXCLUDE = ['powerpc', 'ppc64el', 'ia64', 'sparc', 'armel']

CONTENT_EXCLUDE = ['binary-{arch}', 'installer-{arch}', 'Contents-{arch}.gz', 'Contents-udeb-{arch}.gz', 'Contents-{arch}.diff', 'arch-{arch}.files', 'arch-{arch}.list.gz', '*_{arch}.deb', '*_{arch}.udeb', '*_{arch}.changes']

with open("ubuntu-ports-exclude.txt", 'wb') as f:
	f.write(".~tmp~/\n")
	f.write(".*\n")
	for arch in ARCH_EXCLUDE:
		for content in CONTENT_EXCLUDE:
			f.write(content.format(arch=arch))
			f.write('\n')
