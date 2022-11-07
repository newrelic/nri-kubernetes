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
	"github.com/newrelic/nri-kubernetes/v2/src/client"
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
	clientCACert = []byte(`-----BEGIN CERTIFICATE-----
MIIDVjCCAj6gAwIBAgIUFC4471Vr90q3/UIKSA0/TGWdXUowDQYJKoZIhvcNAQEL
BQAwQzELMAkGA1UEBhMCRVMxDDAKBgNVBAgTA0JDTjESMBAGA1UEBxMJQmFyY2Vs
b25hMRIwEAYDVQQDEwlteS5vd24uY2EwHhcNMjIxMTAyMTI1OTAwWhcNMjcxMTAx
MTI1OTAwWjBDMQswCQYDVQQGEwJFUzEMMAoGA1UECBMDQkNOMRIwEAYDVQQHEwlC
YXJjZWxvbmExEjAQBgNVBAMTCW15Lm93bi5jYTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAM41BhKw+20QcWTg0MUNa7fhZ7U3I+u3BP3QdAfCCJQWGiTE
yMCBAzT7Yk1Fgaq4A90fr6d27rFLemtzB+m73bTc7GBEiBBJRBrkfX5+zv4Dhz5B
K9yqnODBe8rXUxv9cb6bJcYS5l1PKSjtAJnRWYuDMgf+cLQLhNZilLDHlcnpUDbN
eQuwrO+s/lFjW/4LQd/Krfcxye7w5//vuPwcEsv1dACJKeHxoLOVtz3pq523NTne
iaenCtvSYfY1JW6zf4IDx+pSduvpslSvlQFl+l+FqkwuJ+gNGM1e06LcfQ16ULwj
p6zFxzw5hscwC025xGe+4hx/ddTwEOQInI9Sty8CAwEAAaNCMEAwDgYDVR0PAQH/
BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFCCo4Adsf/JScqtOgERb
B5pmyiL/MA0GCSqGSIb3DQEBCwUAA4IBAQBJM48fM1f1/6CExybfLn10M/2Bde6M
A2Kuo0Q4N65HUF+9LdacDxyEygtuLrZ+BIdDVjIq4lbKCnqZM+j/UfxWsCgVMran
QUiGKnlpIW7X8o/IKrQmyMyUIa3LyQLVQWQp6sluYN5DPeQ9pVnjgc5feQHUDY0k
u0Ycn+k79Gibu1m3EdtnymIWv1nIPrAsMwomHZEQuiL2lahPM7wpE/yX6voMSDY6
zi8BOyoQSU/5M+6It/Ch0MgdOM94SGmXGTjjq/q9gCHgkCco1Qk00+54Trc1UZsR
tYY1BVEEikAx4nKW/G8YLMSuzPZuTknCvafC+N552AWqevnShNJkI1d5
-----END CERTIFICATE-----`)
	clientCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDWTCCAkGgAwIBAgIUaynAmwh15Ikcn+uOaVnyL8LPOF0wDQYJKoZIhvcNAQEL
BQAwQzELMAkGA1UEBhMCRVMxDDAKBgNVBAgTA0JDTjESMBAGA1UEBxMJQmFyY2Vs
b25hMRIwEAYDVQQDEwlteS5vd24uY2EwIBcNMjIxMTAyMTMxMTAwWhgPMjEyMjEw
MDkxMzExMDBaMBExDzANBgNVBAMTBmNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAOUCRob9MCOU3e8OouLAw9YGkwmTwQhoaGQ91zee7Fwl7CWC
jiD7Z8DKsLReua8eFIlajBzhrKKv8nUIrn5UPLFucQ8dFFKyQbMKAljuvn2z+JmL
GEBQR+6ArqwuLNcIcjorj4CqbZLMYU+JVqnnEAJktaVVRid/fnvAuFkydosaj5KR
pI/4iBbJfT0oRoNDK3vIcAbzsORwsNx6QIyh/Oy8G/FyWFDeTCtuGYZsiN77bIzC
QAlf1xPW4tLpC+apbtLfFpD+d1cW2XIjT9LIXDg/FjIuX8aKuIkm0QkwKXfXFXqj
ZFg5q267x687ugggegNXnALMjteFV6NVqLbYGq8CAwEAAaN1MHMwDgYDVR0PAQH/
BAQDAgWgMBMGA1UdJQQMMAoGCCsGAQUFBwMCMAwGA1UdEwEB/wQCMAAwHQYDVR0O
BBYEFMZsgsfzxXOtUVhINvwZ+4wfoC32MB8GA1UdIwQYMBaAFCCo4Adsf/JScqtO
gERbB5pmyiL/MA0GCSqGSIb3DQEBCwUAA4IBAQBN69z6fcSEAqpRiWfcZ8Cj6a9P
MBqOj0wUIOVHdJl8ZqK8r4PYdZ8fZFSnFH98yPuefXZQbWUOvKCfLntYU2PxvmSD
jdnCC8n2B1THLHiNMEchK7MwCJzgL9bIzCeIAOeBpL0NxN7zie+FV9gRGKnzS0sP
IIb2//kgqm5B8ATobLmrpCrPFvvONiqoPuIR8MhSmRXYEib/NgKTkc3nO5UsJRms
ucSP1oxG3VYOJ9IkizFN1sRCVLIRNddOBBPGkDw2NxKEIXX9cOPjoD1sCshsigc1
ifv4GchsjkL1qf3Q+eAo51qj2eXMl5H+U5M+1JzujI3w66/d6x+o81gUbk1Z
-----END CERTIFICATE-----`)
	clientKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA5QJGhv0wI5Td7w6i4sDD1gaTCZPBCGhoZD3XN57sXCXsJYKO
IPtnwMqwtF65rx4UiVqMHOGsoq/ydQiuflQ8sW5xDx0UUrJBswoCWO6+fbP4mYsY
QFBH7oCurC4s1whyOiuPgKptksxhT4lWqecQAmS1pVVGJ39+e8C4WTJ2ixqPkpGk
j/iIFsl9PShGg0Mre8hwBvOw5HCw3HpAjKH87Lwb8XJYUN5MK24ZhmyI3vtsjMJA
CV/XE9bi0ukL5qlu0t8WkP53VxbZciNP0shcOD8WMi5fxoq4iSbRCTApd9cVeqNk
WDmrbrvHrzu6CCB6A1ecAsyO14VXo1WottgarwIDAQABAoIBAQCB6aqMxXDbnoXQ
KaNpsyTlc1FSa4lj9abSxuoiWXuIQtMV7FwohbYz/kgD6oC3wP6xdLZrY/KFT/7h
OY2TiMHtfdORWVPAHfN7V8BBJx7VPJVYtTmKsoA74rA0aPVy/w2dxjxgJ06Fqn/B
mQ2a0MOaN/t70UY8/eyI06lAoInzGolc/BxMBgmn5SA3t+16oWISgGZE24pUh0xG
vGk4SD2TUwWFCENDEpyjRCwW54IlQ3TbJUDCltZ1bLJWW+qwkWXY8N+VwdnVgCfr
GoIRlCVHe8jYpqmHBUFq2a+MXHjH004gTM9KKQVvjcfTglHBZTvBZWwDjehx3I95
wQ1TMzMBAoGBAPpvPKXALfQ68Hh+zYuZIr/zv0A1o50KhkQ9dKgiJPDAxoXCFeID
A3g38iEZj79B9UICx1OeWYquXSGmbmzQYBf/TziNfrr+wrn1fL3DyAoB7kGhK0Ri
ZyCmAN739AOvfPB22GDx0NiYeBhGBjTENAk9ll7uO3Mky3XIXS7eNojvAoGBAOoZ
JQir1NbwzX2uCLnSoFptiT5TAEpi4G8yPEFOUMXbZDU/Fv8IcmZmJBIhHxqx9aq/
kI1sObpjY10Wnyn74sBzv4Nn8+pm1IT9FN90TbYnteXRumtZ5r9Xl9j9K2vIE3ug
hV+J/jv6o/r7BYgar0XiKFKuZV6hMB2pxl4UugpBAoGAX5HnwRFP+C4t6q3pXua3
vi0UxTozEBEeIBib1jYBhubqW80vcKrZvh0Lh9orYz+WivRogN6jKStVWywaY+g5
Y68I2noU7OOgCDtIuVpnknoeJGmPC2/KD0mKd4yEUIu90D5qYMSngKDe49SFNcnS
Wdxo8B1WDqDyDCbEeMhQY30CgYAaJaTVSxwCxfKtzvp6huQSNZnWtD6cEF8xDFNe
l/i9oLuYlutioPbmKRJuU/S9bpMZ9zuWEDiCcQdwJk6wycmR5VvGuZ2s2L9z+zCR
pNPpPJY8jShdRTVYudfkDKME7tv+OveqrCcRW/Vk2xTLFu/sxk3qrj/0Sdyt84CM
kZQWAQKBgQCAM7q4IGKqbvz4JlkQFHL6qworTewP78phjEY4taggfNV+LMEM86HV
zoHOOMkKLHdjOA1Pz+tHhUoW7P/npiHqd5Gc+Vj8oykasF7qx1NHfYYUSaeByiUQ
UjjTodSBXpwrvBU298T9VRKsn9ydowSedZYa5Gl7GgnMfW1k1yuuqg==
-----END RSA PRIVATE KEY-----`)
	serverCert = []byte(`-----BEGIN CERTIFICATE-----
MIIDcTCCAlmgAwIBAgIUR7SABjgd0mehGzh5+8ImcZXqYEYwDQYJKoZIhvcNAQEL
BQAwQzELMAkGA1UEBhMCRVMxDDAKBgNVBAgTA0JDTjESMBAGA1UEBxMJQmFyY2Vs
b25hMRIwEAYDVQQDEwlteS5vd24uY2EwIBcNMjIxMTAyMTMwOTAwWhgPMjEyMjEw
MDkxMzA5MDBaMBExDzANBgNVBAMTBnNlcnZlcjCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAMh+TA7FdKmjuGtIg/UyhVrbaH4QIjoIba0l1C0WjsNJyU+C
nCBTLWjcsgPPoDpHcRxZrOeuC7JUDCjKwvgwxnFyslGMFUAtiekO+c3u6gP14Hrz
T4/LPlzcM1ceXyLvstFG3pMdIbTt6K1jzcDkR842yyv+K8kksCpPexB1vMWyjA8M
AiBVCkxRc2f27AmxFhtiaVT/NxAwbioRpMvwbzsDxgUIjpQCE/HizzaR7TOX0ndK
b0azU9+AjueKYseOwAWfqD05xqICCtwCzjgM64XLdn9wc2Tlp/gq0SgXIo5JxDSu
QRc1vvQaOFbzGBD+w8bTMeJLuFSx/wHDhG2sOBECAwEAAaOBjDCBiTAOBgNVHQ8B
Af8EBAMCBaAwEwYDVR0lBAwwCgYIKwYBBQUHAwEwDAYDVR0TAQH/BAIwADAdBgNV
HQ4EFgQUNNsNQp3D7I1tBbagvhFOVvsU3YwwHwYDVR0jBBgwFoAUIKjgB2x/8lJy
q06ARFsHmmbKIv8wFAYDVR0RBA0wC4IJbG9jYWxob3N0MA0GCSqGSIb3DQEBCwUA
A4IBAQAbA2Zu41dMpOAc351hLlSopmb0fdF4ASqpuvj9Yo66N/nEv4SoSvAMQBE7
Xcc5plG4WtdFxUr9peQw9FTfbcB8ML72+F9gjfzgydsLg+JMBzwDuL/bPF46Ne8N
BaGZOgOFNJqO53kNxzb9dpwPowaQmICR8k88TAj5BtIaOkUkGVUxa1Bna/x0c+Ku
hhJ8rnO7bjIAPBCVKi40iTYgGkvqDcDaLmkVF42ssasTcWOhOylAl8v+wtg1N7sy
oNlwDXrGyfY7e3d24DuoUUgmK80EEqAPsz5fh5Wh1LOcHELP1x/EeBVi2jdjJ1/D
/b8iB3J6Aqyl+fZGlTLUEatQVPkv
-----END CERTIFICATE-----`)
	serverKey = []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIEpgIBAAKCAQEAyH5MDsV0qaO4a0iD9TKFWttofhAiOghtrSXULRaOw0nJT4Kc
IFMtaNyyA8+gOkdxHFms564LslQMKMrC+DDGcXKyUYwVQC2J6Q75ze7qA/XgevNP
j8s+XNwzVx5fIu+y0Ubekx0htO3orWPNwORHzjbLK/4rySSwKk97EHW8xbKMDwwC
IFUKTFFzZ/bsCbEWG2JpVP83EDBuKhGky/BvOwPGBQiOlAIT8eLPNpHtM5fSd0pv
RrNT34CO54pix47ABZ+oPTnGogIK3ALOOAzrhct2f3BzZOWn+CrRKBcijknENK5B
FzW+9Bo4VvMYEP7DxtMx4ku4VLH/AcOEbaw4EQIDAQABAoIBAQCFvmpyOBn4x/RP
7NHKEWeQEmkEHzMVz2WKaX++jBuz/lbCKXiIv7O9DevaSvixp9K2fMOw0ROQZCyw
UYH6Gl9mcoKtj2rlovsqcwkE7OlCtxSGMCTU4Vm6jFHbPbFtFsUMgeAb9wTzMvlS
IQ+yKxYTY83ojOcciNLThq2rbz78CUzdE7nJtVdPEfuj1yzxSMFYDdn3odaCr+on
DnzeHYZdazt8LxjJWKoTHpU6SchKDZL2RnUDSmENvnHR5VyKgmEDw1oTv6EhseAL
D2ByxgiUkNydy5LW0iYmQdaTNkI/NbViZuGIx63BbFoV5Hor/4HBr+svdzZRqWsa
6y4XSCc5AoGBAP29ZCGZDkU2toHun3OOdmTHXkBX04hSp/KnXw2X+uCgZ46ARiKZ
nceU2AL1PMwnX5fy594f6MRqeOj5p6CAv44pIJ8Pik6v+wQ5RK9D55EdeAvnfBGb
YeBkjtR6aioqB2yDdzrKme6TFy188gg685EDdmfZYRgqW3iKhhyUnF7bAoGBAMpH
fJ8omD2Cs4h4bGT0qrJyn3cVcBaSHcxWS73J3/VXvYfTxeT3WZvxnwP/GTjGqpKu
fbHf7dJLJBwGdg0IB95/olvCzwaSMt0CAbvcyl7ysxd/RwyYkSL7XGOdnUq23y2u
xwRVbpTR5c1FgABozS0Qe+y9SCSC1L8Zqfbyc2qDAoGBALG0FA5brNzYZpU000MQ
wOXvopiZabINgUW15iIVEESE0kHAoF3XC+Mc4POhYMTxxkcafTzZSCFXF/rB7Z3A
zWb4crozHf/hy4C3wtykR+cfplVf90o1ciS/CDDS0stYx/49TCFGhuvI4/Cdkrwk
3TPwItq0KQXNlGYlTatygNkFAoGBAMfVsedm6mgyPH2BQszF7fEXTjUOR8r0lV2u
j2szCf9OrB6I+AOI3c0y+j6vgVJW6mK44dKdgEz2EPli5LNhEK0eeN6gaXh7bKZs
ehwHNyJwML/w7Ncjzpa5rv920dLjMT7nYRQF9pYtexK9K4S8BJ8Vnug14xS278jP
aNtfkOhTAoGBAIm977NFmqvOm4tMPEFfmhByBY/X6BNsiI3npfPkoZoa3OFgfAFN
M8vxw5NybuI4Lpr+TcZrZHpxZ1s+dKjWWKz7OqhaaReC6428yUil7y6EY9rug2Al
kh49/RIKPyyRA6vfmozzS8h03LA070w7oZtolpZVt4ImjtebS8d6At3H
-----END RSA PRIVATE KEY-----`)
	serverCACert = []byte(`-----BEGIN CERTIFICATE-----
MIIDVjCCAj6gAwIBAgIUFC4471Vr90q3/UIKSA0/TGWdXUowDQYJKoZIhvcNAQEL
BQAwQzELMAkGA1UEBhMCRVMxDDAKBgNVBAgTA0JDTjESMBAGA1UEBxMJQmFyY2Vs
b25hMRIwEAYDVQQDEwlteS5vd24uY2EwHhcNMjIxMTAyMTI1OTAwWhcNMjcxMTAx
MTI1OTAwWjBDMQswCQYDVQQGEwJFUzEMMAoGA1UECBMDQkNOMRIwEAYDVQQHEwlC
YXJjZWxvbmExEjAQBgNVBAMTCW15Lm93bi5jYTCCASIwDQYJKoZIhvcNAQEBBQAD
ggEPADCCAQoCggEBAM41BhKw+20QcWTg0MUNa7fhZ7U3I+u3BP3QdAfCCJQWGiTE
yMCBAzT7Yk1Fgaq4A90fr6d27rFLemtzB+m73bTc7GBEiBBJRBrkfX5+zv4Dhz5B
K9yqnODBe8rXUxv9cb6bJcYS5l1PKSjtAJnRWYuDMgf+cLQLhNZilLDHlcnpUDbN
eQuwrO+s/lFjW/4LQd/Krfcxye7w5//vuPwcEsv1dACJKeHxoLOVtz3pq523NTne
iaenCtvSYfY1JW6zf4IDx+pSduvpslSvlQFl+l+FqkwuJ+gNGM1e06LcfQ16ULwj
p6zFxzw5hscwC025xGe+4hx/ddTwEOQInI9Sty8CAwEAAaNCMEAwDgYDVR0PAQH/
BAQDAgEGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFCCo4Adsf/JScqtOgERb
B5pmyiL/MA0GCSqGSIb3DQEBCwUAA4IBAQBJM48fM1f1/6CExybfLn10M/2Bde6M
A2Kuo0Q4N65HUF+9LdacDxyEygtuLrZ+BIdDVjIq4lbKCnqZM+j/UfxWsCgVMran
QUiGKnlpIW7X8o/IKrQmyMyUIa3LyQLVQWQp6sluYN5DPeQ9pVnjgc5feQHUDY0k
u0Ycn+k79Gibu1m3EdtnymIWv1nIPrAsMwomHZEQuiL2lahPM7wpE/yX6voMSDY6
zi8BOyoQSU/5M+6It/Ch0MgdOM94SGmXGTjjq/q9gCHgkCco1Qk00+54Trc1UZsR
tYY1BVEEikAx4nKW/G8YLMSuzPZuTknCvafC+N552AWqevnShNJkI1d5
-----END CERTIFICATE-----`)
)
