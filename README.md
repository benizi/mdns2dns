# tinytld

Tiny DNS server to run a mini-TLD.

# Disclaimer

This probably flies in the face of several DNS-related RFCs, but it's
super-useful for a *non-public* network.

# Current state

Currently, it doesn't do anything with .local names.  It just lets you:

1. Register your name at whatever.in.host

2. Query registered names as (any.leading.parts.)whatever.host

3. Interact with .host names over HTTP (add/list) on a specified port.


## Example

```sh
$ dig +short newname.in.host
$ dig +short aardmark.newname.host
192.168.30.19 # should return a non-loopback IP of the current machine
```

# Utility

If you set Pow up to handle the domain `newname.host`, you can serve your apps
as `appname.newname.host`.  Per the [Pow User's
Manual](http://pow.cx/manual.html#section_3.1), set your POW_EXT_DOMAINS
variable to `newname.host`.

# Disclaimer

This is mainly an excuse for me to play with Go.
