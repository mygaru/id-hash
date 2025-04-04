# gpsi

## Implemeting Your Own GPSI

TBW.

## Deploy

Notes:
- Important to set the address of your UidMap instance (without any path)!
- Only the IP of your AuthMW instance should be allowed to make requests here! Set `httpAuthAllowedRemoteIPs` as the IP of your AuthMW instance.
- Makefile run command can be found at ./cmd/gpsi/Makefile.

Config example:
```ini
[http]
httpServerListenAddr        = :8000
httpServerCompressLevel     = 1
httpServerWriteTimeout      = 10s
httpServerReadTimeout       = 10s
httpServerReduceMemoryUsage = true
httpServerKeepAlivePeriod   = 30s
httpServerListenBacklog     = 65535
httpServerGetOnly           = false
httpServerConcurrency       = 400000
# 512 MB
httpServerMaxRequestBodySize = 536870912

[common]
httpServerName              = MyGaru GPSI
; allow only auth MW IP to access!
httpAuthAllowedRemoteIPs    = 127.0.0.1,::1


[log]
; DEBUG, INFO, WARN, ERROR
logLevel = INFO

[pim]
uidMapUri = http://localhost:8000
pimTimeout = 5m
```

