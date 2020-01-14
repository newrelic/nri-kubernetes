package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"testing"

	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-kubernetes/src/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
)

const (
	testString = "hello, MTLS world!"
	secretName = "my-tls-config"
)

func boolPtr(b bool) *bool { return &b }

func TestMutualTLSCalls(t *testing.T) {
	tt := []struct {
		name               string
		insecureSkipVerify *bool
		cacert, key, cert  []byte
		assert             func(*testing.T, *http.Response, error)
	}{
		{
			name:   "Successful call with proper configuration, should succeed",
			cert:   clientCert,
			key:    clientKey,
			cacert: serverCACert,
			assert: func(t *testing.T, resp *http.Response, err error) {
				require.NoError(t, err, "request should not fail (i.e. non-2xx response)")
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				require.NoError(t, err, "error reading response body")
				assert.Equal(t, string(bodyBytes), testString, "expected body contents not found")
			},
		},
		{
			name:               "InsecureSkipVerify should not check the server certificate, so no CaCert is needed.",
			insecureSkipVerify: boolPtr(true),
			cert:               clientCert,
			key:                clientKey,
			// no cacert...
			assert: func(t *testing.T, resp *http.Response, err error) {
				require.NoError(t, err, "request should not fail (i.e. non-2xx response)")
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				require.NoError(t, err, "error reading response body")
				assert.Equal(t, string(bodyBytes), testString, "expected body contents not found")
			},
		},
		{
			name:               "InsecureSkipVerify or CaCert should be set",
			insecureSkipVerify: boolPtr(false),
			cert:               clientCert,
			key:                clientKey,
			assert: func(t *testing.T, resp *http.Response, err error) {
				// todo: check if it's really the correct error
				require.Error(t, err)
			},
		},
		{
			name: "No config should fail",
			assert: func(t *testing.T, resp *http.Response, err error) {
				// todo: check if it's really the correct error
				require.Error(t, err)
			},
		},
	}

	for _, test := range tt {
		t.Run(test.name, func(t *testing.T) {
			endpoint := startMTLSServer()
			c := createClientComponent(endpoint, test.cacert, test.key, test.cert, test.insecureSkipVerify)
			resp, err := c.Do("GET", "/test")
			test.assert(t, resp, err)
		})
	}
}

func createClientComponent(endpoint string, cacert, key, cert []byte, insecureSkipVerify *bool) *ControlPlaneComponentClient {

	// Data will be the contents of the secret holding our TLS config
	data := map[string][]byte{}

	if len(cacert) > 0 {
		data["cacert"] = cacert
	}
	if len(key) > 0 {
		data["key"] = key
	}
	if len(cert) > 0 {
		data["cert"] = cert
	}
	if insecureSkipVerify != nil {
		// this changes a bool to a byte array containing `true` or `false`
		data["insecureSkipVerify"] = []byte(fmt.Sprintf("%t", *insecureSkipVerify))
	}

	c := new(client.MockedKubernetes)
	c.On("FindSecret", secretName).
		Return(&v1.Secret{
			Data: data,
		}, nil)

	return &ControlPlaneComponentClient{
		httpClient:               &http.Client{},
		tlsSecretName:            secretName,
		authenticationMethod:     mTLS,
		logger:                   log.New(true),
		IsComponentRunningOnNode: true,
		k8sClient:                c,
		endpoint: url.URL{
			Scheme: "https",
			Host:   endpoint,
		},
		nodeIP:  "asd",
		PodName: "asd",
	}
}

func startMTLSServer() string {

	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	cert, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		panic(err)
	}

	endpoint := fmt.Sprintf("localhost:%d", l.Addr().(*net.TCPAddr).Port)

	clientCAs := x509.NewCertPool()
	clientCAs.AppendCertsFromPEM(clientCACert)

	m := http.NewServeMux()
	m.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, testString)
	})
	server := &http.Server{Addr: endpoint, Handler: m}
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    clientCAs,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		NextProtos:   []string{"http/1.1"},
	}

	config.BuildNameToCertificate()

	tlsListener := tls.NewListener(l, config)

	go func() {
		logrus.Fatal(server.Serve(tlsListener))
	}()

	return endpoint
}

// These certificates are taking from the etcd TLS example
var (
	clientCACert = []byte(`
-----BEGIN CERTIFICATE-----
MIID3jCCAsagAwIBAgIUKXbvWUAgVnL7iVUcet3e4x1qH70wDQYJKoZIhvcNAQEL
BQAwdTELMAkGA1UEBhMCVVMxFjAUBgNVBAgTDVNhbiBGcmFuY2lzY28xCzAJBgNV
BAcTAkNBMRgwFgYDVQQKEw9NeSBDb21wYW55IE5hbWUxEzARBgNVBAsTCk9yZyBV
bml0IDExEjAQBgNVBAMTCU15IG93biBDQTAeFw0xNzEwMzEyMjUzMDBaFw0yMjEw
MzAyMjUzMDBaMHUxCzAJBgNVBAYTAlVTMRYwFAYDVQQIEw1TYW4gRnJhbmNpc2Nv
MQswCQYDVQQHEwJDQTEYMBYGA1UEChMPTXkgQ29tcGFueSBOYW1lMRMwEQYDVQQL
EwpPcmcgVW5pdCAxMRIwEAYDVQQDEwlNeSBvd24gQ0EwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDgwydE2HYdqTiz//bMSL/C4w2y4DDMjGZgNdo50VIl
QniiNDrPRB8Xt1fY4MO3VAyLWU934YKssrsqSDn1PE/Fcc5yURKaMc+rsSlGr8Qn
E/W551OuEIAKujPKhIIBk6X4mBVQWEQnjVskAD0aEjYtoo4I/+9F67Rklub5fXwE
ESsB5yf812zWSzC51Ls0s1Uc80h5buh4p7HtFDOY0oCNxNx2Ou21xn5qqpG/1flY
ReHHKmuvRWwnxQdQu+qrill8j/H48Ly6ZGSV47Qqiw7Hb2JK2vnsf95Pp8nEProU
53M5V5y5WHW8VH0sVgzjgc0rC0w0TCCQVkGUSttqFpdJAgMBAAGjZjBkMA4GA1Ud
DwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgECMB0GA1UdDgQWBBSgWCYJFFoa
6O22U4MaelWJt3khUDAfBgNVHSMEGDAWgBSgWCYJFFoa6O22U4MaelWJt3khUDAN
BgkqhkiG9w0BAQsFAAOCAQEA1ELpWokOl1kwD5fbuROUZ9YedhXVRBWUKKluqQCr
eUUU7x/txKZ4xRYr3s1ltuUjxOMs5XbJSJq1z3tifDQ1srDjyU2CkKtZfjX5xmaS
QHCEJv/WgC6SBHGVYAgZ1hONPN2WpWxDYOLf6seonLszCHLkHMmjub8uFi/TSP8x
5OQ2SYLpHQDQcb3xlwk6+09ZuihAzWAgNAOvW+cNrunlD7N+BBTWMZmugKzqk0BT
avTn+p4dimFk528Iz+bk2uCfmF9WlnHm9DmlwCwM4PioGND7ag1VXAsgkqRWGa3k
uCP+NP3PpnGJLfxV5u20YlNLJk8bVFMB6FoFMafREVMQBA==
-----END CERTIFICATE-----
`)
	clientCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDzzCCAregAwIBAgIUFr+6DAbtFSnfqm4Aup/yPagMWB8wDQYJKoZIhvcNAQEL
BQAwdTELMAkGA1UEBhMCVVMxFjAUBgNVBAgTDVNhbiBGcmFuY2lzY28xCzAJBgNV
BAcTAkNBMRgwFgYDVQQKEw9NeSBDb21wYW55IE5hbWUxEzARBgNVBAsTCk9yZyBV
bml0IDExEjAQBgNVBAMTCU15IG93biBDQTAeFw0xNzEwMzEyMjUzMDBaFw0yMjEw
MzAyMjUzMDBaMEgxCzAJBgNVBAYTAlVTMRYwFAYDVQQIEw1TYW4gRnJhbmNpc2Nv
MQswCQYDVQQHEwJDQTEUMBIGA1UEAxMLZXRjZCBjbGllbnQwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDBA3WhHLwWPFeaqsDIvwkqnszPIiCpyDdLkpTx
XcjM6PTiA/hA2y3WRlcQrBsPAcGyj3V+fGxOCGTIktoKLvFk4GjGR6zw+hfIGwSe
hbuPAQnkaoCsctrgeRjyv7TUb9N4KzXOYfP/RHAtZxh+91gmo/oF/kgzJz+MFR/y
OodBzzdXp7ZAumt0HUB5kqDxQDNXftnquK0WWvjU9geoYwFuHZ4J25p18RmMkL7p
hAWK+MB8+DgTMDP3SGh7SwdVS41UJhJTxK6C/ebj5fMjMNmsinAtCt39pvgwcP7y
p3k/IPoXxWBqRaC3NZW8Mq/dFVMVdcDZ9kzXWiRuvVnCpwzzAgMBAAGjgYMwgYAw
DgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMCMAwGA1UdEwEB/wQC
MAAwHQYDVR0OBBYEFNJTsiGVvKItIERnVbqCz9kpQoDyMB8GA1UdIwQYMBaAFKBY
JgkUWhro7bZTgxp6VYm3eSFQMAsGA1UdEQQEMAKCADANBgkqhkiG9w0BAQsFAAOC
AQEAjleSdnxhceY4/muz9HsC1Fk9Yh/KqkMZWMbKCSGyDGR27hS69RJ5VSpfHV6O
2BzXrhH7ygbnqGIl9BreBVOuYsNK/Z2l/h1vwRmehpjlht1LWQAYgSsS24RxgFSL
w2QREI+3hYtnOW7ESnnZTD5xlNJenR1iQx7+R+1Y4R2viqN6s8WF30p7q28EzBVR
+jlqezpwDhmIRTv91IQhrhmkn/xFtg0ZiIS/AGbjKpPKmVw19mweh2TRscV1V8Gk
0nCV8xvREELZyFLNyXdlkhVMyE2f0Dp4uYcPwuPlGTFcQOmELirzi0Gw94WaoIPO
58JdskKHXTJdzbV15WYM/KD8Kw==
-----END CERTIFICATE-----`)
	clientKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAwQN1oRy8FjxXmqrAyL8JKp7MzyIgqcg3S5KU8V3IzOj04gP4
QNst1kZXEKwbDwHBso91fnxsTghkyJLaCi7xZOBoxkes8PoXyBsEnoW7jwEJ5GqA
rHLa4HkY8r+01G/TeCs1zmHz/0RwLWcYfvdYJqP6Bf5IMyc/jBUf8jqHQc83V6e2
QLprdB1AeZKg8UAzV37Z6ritFlr41PYHqGMBbh2eCduadfEZjJC+6YQFivjAfPg4
EzAz90hoe0sHVUuNVCYSU8Sugv3m4+XzIzDZrIpwLQrd/ab4MHD+8qd5PyD6F8Vg
akWgtzWVvDKv3RVTFXXA2fZM11okbr1ZwqcM8wIDAQABAoIBAQCF26tZl/8NgL3U
wzU+Q9bMmyM5R9bVSMiofbkUB9G54pnqoYwrFpacc13wbxu49aPq/TkkBpBqMcIL
pGTZCSNarZOcZ5sV6KxTmAFFG0Qvci31HrOsZV9MrE9UEwYLCp7jSTxgrGg2kbUm
l8hSTaHx8mj0fRx/dWnJ8eCc8mBZj3LO6A9w1bSa53LGh5LggyxDFwRfECXcUEAI
RWFkP8QWVweSzbScHomVEQ3Wn0isBgMF3s5hcdvu6txkS+p12/M7ZXk1cgXZTNtM
6WUhX/uP/NrcA4b5NIRiW5hQ0qgGsCcVD7Tq3fBHZbt6dfRsPE+XIkQF79lmIRIu
DYeamONxAoGBANktHJ25DJ9lB99kIF8wTm1F0Xv1rfkz2YrW38MVhOErDfDurgTk
4VTcIfV8gNTmI/LLIOvk9B2nzm2wWJWFO2XRdEuTbWjQQn/po8G5mEKieYiHFHY1
0vx2HrwzZAtuxz0ceHC6aEoYg76w4a7ILiSw4eoMsIBjci7Q8kO3HUPJAoGBAOOE
j812IJylsdPiYAFlnqmm4+XPqXnXA8juUzlZr69GTgtwb0o6ZmnvmNPwErBXPwaB
DzWauENu5Tg1PxBjxBCxCjDlJcizhbGNaGLn8TphVpzg3roIWEqn6LR8es7WGao1
H9TyUSKGCSqCnszVhdBw+DyFxnKCKi8VXbHfFZDbAoGBAI6POlWef1aybzSI+WcC
wrigOB7y6rzG+GpXGpNosM1OAdzCEKFNzUxzJCeNDtSyLa7XAElZBZXh7XO7aqrb
xl3T3E8v+4XuD3j/2Wr1daloFfc1FI10T4dB0nMgGPAYS9klsznsY0EgTnsCiWK+
LOwQ4HtO0R22KeHpbt5ceW1hAoGBAJ3TWUX3ycugjWkkQeD2M0gQg0rp8PCaHQAH
gyfndR2rMXxx9GGTfXPDR0rN4Mj+3LOQV5Khz2zHwq5pEWQ3MM07YoxkiP9euUFf
jKf/qbEL0N9mhlqaa1TugViieTZ+ArO1wm0f4vSF8lnQ3oPNItRjaW/ihLTuYoDi
22oGDJm9AoGAS7rQk9SkMvg0miOGJJSKPwhLjvPRLxYWE/X9DkSpJPLdlay7LXj2
o9kytsNQ305U+aB55h1MkpI22TxClFIJ0YiPHOoYuvvEJ6YCpphZiNdRQVT/8Unv
sCV40oCU+RDsJNC4FAuVnXAovdT1VHkhZum7i25oto+C0hdZGpSlloM=
-----END RSA PRIVATE KEY-----`)
	serverCert = []byte(`-----BEGIN CERTIFICATE-----
MIIECzCCAvOgAwIBAgIUJ9A0ORmRWaag95KE7Mw7ZEeAsqwwDQYJKoZIhvcNAQEL
BQAwdTELMAkGA1UEBhMCVVMxFjAUBgNVBAgTDVNhbiBGcmFuY2lzY28xCzAJBgNV
BAcTAkNBMRgwFgYDVQQKEw9NeSBDb21wYW55IE5hbWUxEzARBgNVBAsTCk9yZyBV
bml0IDExEjAQBgNVBAMTCU15IG93biBDQTAeFw0xNzEwMzEyMjUzMDBaFw0yMjEw
MzAyMjUzMDBaMEgxCzAJBgNVBAYTAlVTMRYwFAYDVQQIEw1TYW4gRnJhbmNpc2Nv
MQswCQYDVQQHEwJDQTEUMBIGA1UEAxMLZXRjZCBzZXJ2ZXIwggEiMA0GCSqGSIb3
DQEBAQUAA4IBDwAwggEKAoIBAQDHQ27rtTF+U5X67gpWf4tmAnG9P/p2PzVltqWi
jn2niHcAm3rA2ZyKa97REIg43j3K9+mfcah36fY6h567OfxuOU3MY5Nf7A/EchxZ
qqiGCyghwXBWs7kspucxr3KIrlZD8ZFshLDKwKJuHonxBqZgU90gCGrex+RnYRO3
fUBWXTua/d+k5kKXSrpSYQhevZX3QxPwhBYc1a6tEgNcTi4hQWjNoVNVq0vySps3
5eOzh/vD7Y6iFimp6EkRDfqfEUG9Vt1ngfILqP/P3chHVFkBPGLWYqGrD7jELmry
DtHfbFUuzZJcz4I5FxE6V0LdkOEbI7VGqLcCFokwzi3gCkF9AgMBAAGjgb8wgbww
DgYDVR0PAQH/BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMBMAwGA1UdEwEB/wQC
MAAwHQYDVR0OBBYEFAt3OUO1pMZ4rQSzKbebIWE/PbYEMB8GA1UdIwQYMBaAFKBY
JgkUWhro7bZTgxp6VYm3eSFQMEcGA1UdEQRAMD6CFSouZXhhbXBsZS5kZWZhdWx0
LnN2Y4IaZXhhbXBsZS1jbGllbnQuZGVmYXVsdC5zdmOCCWxvY2FsaG9zdDANBgkq
hkiG9w0BAQsFAAOCAQEAjdkgFCI0ySHIf3QyfckcoCpPVTU+S3g4gRp+RWGbQ2yp
pgAJBBG6tFZRu9VGhj2uDtqDxyp1igKs4aOA95Amm92/k9y8Xw1LMPNIdmst/1ol
UbLrFfhxwYbZTpn0FDESHrRNX8j5UlRFsoqcW6CXdem8DL2MB5eshnpKZMLABoqM
EzNRvfnJ4tkH9T949nJkjXcih/0UWg/S/P9MsXMxbXpuDxivynMcBUQxPfdkMzTV
BwogyJn/f508ahlmlwb0JM5D7RLhHndBP1hbs00KE8g927PkpL8QNBW/DL2bjS0y
UI/MmAWRZiP6LAj+3H12QbbCmG5LvY+Ujbz28qSuXg==
-----END CERTIFICATE-----`)
	serverKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEAx0Nu67UxflOV+u4KVn+LZgJxvT/6dj81Zbaloo59p4h3AJt6
wNmcimve0RCION49yvfpn3God+n2Ooeeuzn8bjlNzGOTX+wPxHIcWaqohgsoIcFw
VrO5LKbnMa9yiK5WQ/GRbISwysCibh6J8QamYFPdIAhq3sfkZ2ETt31AVl07mv3f
pOZCl0q6UmEIXr2V90MT8IQWHNWurRIDXE4uIUFozaFTVatL8kqbN+Xjs4f7w+2O
ohYpqehJEQ36nxFBvVbdZ4HyC6j/z93IR1RZATxi1mKhqw+4xC5q8g7R32xVLs2S
XM+CORcROldC3ZDhGyO1Rqi3AhaJMM4t4ApBfQIDAQABAoIBADYtnIwUAPgDDAVl
EYSBO0qqIXi+W4ApIYCdT53KNloF3a1ZmN+0iz6Lo9KeNxuXOZ/lFi1W/uJTx7IU
S9FGK99gT0niTSDIk2TrTdAHebiwceHzsXKxfQip/LRiqraFCEmC9fJWhacrBz7/
qKvTDguk4bui7kPSf8Sn/W9na8XPKssoPEzYR/sPUh3D8TcdrWZrS2R+6xaLr2Pm
Hi5ZtF9eW4n2biRxXvIFGI76BrFE8UlHcdau7YdTrHQhbLz+go43T02iV4eKdSzp
A3OJDyKbut4fTPWx4rlk1ysaianpdXDaFxXzZd7vXvLq5VfxO17D3Ti9uhjBQE9J
YzUbFPUCgYEA6s9aWhmmMtzoEBt2FngjfDrRxI2Yp0yASvh2bDCO+b5/P8F0uLKU
HcOmJG1949212nPQ9SDnRSSM9joRA3HKP/gRx8KGKoFsQadMnW7tOnEwKJvDWe6V
DypW36zI4YicJJOX+UbLHStom24Y7O7q59wp726p8piy3Sa/MHz2ZAcCgYEA2T7f
oBP76IuTDKOzk3QO0ow7fsVQCM28h2JLMYFnGQQksDvFgdeM4Y8dONrVVahXVH+N
7CAnZ5H3wFEXpXetyuwplEmhfzDnyERvi0udxmYrug0xOu+DCEJP3w7T4pHCDEho
Ea3sbwTmZC4ckfkR/gQKR8RxX2R/6n93rDj29VsCgYEAt3teA+flCfu6ztNWnEo2
mF2yCuAGeDx8R5kNmI79OkRUVPKLjcPln7iBfBee9s8JynETyGh0r3/XMpS/NKzX
ONNUuX7UriRB/q+HW8IRV8iYtDK7HOwkyBvylIgE1M+WC7LVX3GlR97iuAn5KjOr
lZBhqHoWDL6rjco4PeB3/EMCgYEAvOCIJrIZOzZWdA/DqjimRnI7q90611ygRAi2
nWUHUN2kVECzWE8iolz+KBdCkYWZ39JCfv/5ondrMp6Oc4NY62tmPxHBQkcvzZOK
c04b74mXDNw5aCcjAkQ9Ew7eM0dMsccmC/Dt9hwJfyIEHvmwpeu3UGw/sZM8D5Ih
Zu/j7q8CgYAv76kxhx/jxdX77FNflAhM7HK+hpcpJPzy/e/Jle+jbMki6d5lytCp
DjRPVsgVUV6FGOSLRcDzbOyfwVRJduToLox2UUGVJaPRid8D/Y5+HGs2yyKG7ubz
SzH3mMv+TDPFmVsKlECMUgr1aEKJRQf2KcLFioXZlLAIRuzDufnCnw==
-----END RSA PRIVATE KEY-----`)
	serverCACert = []byte(`-----BEGIN CERTIFICATE-----
MIID3jCCAsagAwIBAgIUKXbvWUAgVnL7iVUcet3e4x1qH70wDQYJKoZIhvcNAQEL
BQAwdTELMAkGA1UEBhMCVVMxFjAUBgNVBAgTDVNhbiBGcmFuY2lzY28xCzAJBgNV
BAcTAkNBMRgwFgYDVQQKEw9NeSBDb21wYW55IE5hbWUxEzARBgNVBAsTCk9yZyBV
bml0IDExEjAQBgNVBAMTCU15IG93biBDQTAeFw0xNzEwMzEyMjUzMDBaFw0yMjEw
MzAyMjUzMDBaMHUxCzAJBgNVBAYTAlVTMRYwFAYDVQQIEw1TYW4gRnJhbmNpc2Nv
MQswCQYDVQQHEwJDQTEYMBYGA1UEChMPTXkgQ29tcGFueSBOYW1lMRMwEQYDVQQL
EwpPcmcgVW5pdCAxMRIwEAYDVQQDEwlNeSBvd24gQ0EwggEiMA0GCSqGSIb3DQEB
AQUAA4IBDwAwggEKAoIBAQDgwydE2HYdqTiz//bMSL/C4w2y4DDMjGZgNdo50VIl
QniiNDrPRB8Xt1fY4MO3VAyLWU934YKssrsqSDn1PE/Fcc5yURKaMc+rsSlGr8Qn
E/W551OuEIAKujPKhIIBk6X4mBVQWEQnjVskAD0aEjYtoo4I/+9F67Rklub5fXwE
ESsB5yf812zWSzC51Ls0s1Uc80h5buh4p7HtFDOY0oCNxNx2Ou21xn5qqpG/1flY
ReHHKmuvRWwnxQdQu+qrill8j/H48Ly6ZGSV47Qqiw7Hb2JK2vnsf95Pp8nEProU
53M5V5y5WHW8VH0sVgzjgc0rC0w0TCCQVkGUSttqFpdJAgMBAAGjZjBkMA4GA1Ud
DwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgECMB0GA1UdDgQWBBSgWCYJFFoa
6O22U4MaelWJt3khUDAfBgNVHSMEGDAWgBSgWCYJFFoa6O22U4MaelWJt3khUDAN
BgkqhkiG9w0BAQsFAAOCAQEA1ELpWokOl1kwD5fbuROUZ9YedhXVRBWUKKluqQCr
eUUU7x/txKZ4xRYr3s1ltuUjxOMs5XbJSJq1z3tifDQ1srDjyU2CkKtZfjX5xmaS
QHCEJv/WgC6SBHGVYAgZ1hONPN2WpWxDYOLf6seonLszCHLkHMmjub8uFi/TSP8x
5OQ2SYLpHQDQcb3xlwk6+09ZuihAzWAgNAOvW+cNrunlD7N+BBTWMZmugKzqk0BT
avTn+p4dimFk528Iz+bk2uCfmF9WlnHm9DmlwCwM4PioGND7ag1VXAsgkqRWGa3k
uCP+NP3PpnGJLfxV5u20YlNLJk8bVFMB6FoFMafREVMQBA==
-----END CERTIFICATE-----`)
)
