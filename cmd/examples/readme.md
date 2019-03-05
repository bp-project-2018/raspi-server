Configuration Examples
======================

This directory contains several example configuration files that can be used with the other programs.

Command line tools accept the `-config` option with a path to one of the configuration files.

The `server` command uses one of these files as network configuration.

Configuration File Format
-------------------------

```js
{
	"host-addr": "kalliope",     // address of this host
	"use-time-server": "kronos", // address of time server to use, can be omitted to use local time
	// "host-time-server": true, // if set to true, host will answer time requests
	"partners": {
		// for each partner (other host) that you want to communicate with:
		"kronos": {                                         // address of partner
			"key": "a9da5962eba6ce63efb6eda18cae2123",      // shared key with partner (hexadecimal notation)
			"passphrase": "Ich haette gerne einen Whopper." // shared passphrase with partner
		},
		"shredder": {
			"key": "04801ce16b945ab05986dcf94dc82c2f",
			"passphrase": "There are 69,105 leaves in a pile."
		}
	}
}
```
