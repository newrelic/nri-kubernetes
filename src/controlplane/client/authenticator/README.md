# Certificates generation for authenticator tests

Testing certificates where generated using [cfssl](https://github.com/cloudflare/cfssl) tool.

## Steps to generate new certificates

Install `cfssl` and generate configuration files as follows:

**ca-config.json**:

```json
{
    "signing": {
        "default": {
            "expiry": "876000h"
        },
        "profiles": {
            "server": {
                "expiry": "876000h",
                "hosts": [
                    "localhost"
                ],
                "usages": [
                    "signing",
                    "key encipherment",
                    "server auth"
                ]
            },
            "client": {
                "expiry": "876000h",
                "usages": [
                    "signing",
                    "key encipherment",
                    "client auth"
                ]
            }
        }
    }
}
```

**ca.csr.json**:

```json
{
    "CN": "my.own.ca",
    "hosts": [
    ],
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "ES",
            "ST": "BCN",
            "L": "Barcelona"
        }
    ]
}
```

**server.json**:

```json
{
    "CN": "server",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "hosts": ["localhost"]
}
```

**client.json**:

```json
{
    "CN": "client",
    "key": {
        "algo": "rsa",
        "size": 2048
    }
}
```

Generate the CA certificate:

```bash
cfssl gencert -initca ca-csr.json | cfssljson -bare ca
```

Generate the server certificate:

```bash
 cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=server server.json | cfssljson -bare server
```

Generate the client certificate:

```bash
 cfssl gencert -ca=ca.pem -ca-key=ca-key.pem -config=ca-config.json -profile=client client.json | cfssljson -bare client
```
