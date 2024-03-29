// This code is available on the terms of the project LICENSE.md file, also
// available online at https://blueoakcouncil.org/license/1.0.0.

// As copied from
// https://github.com/decred/dcrdex/blob/master/client/core/certs.go with some
// minor changes.

package dcrdex

const (
	decredDEXServerMainnet = "dex.decred.org:7232"
	decredDEXServerTestnet = "bison.exchange:27232"
	decredDEXServerSimnet  = "127.0.0.1:17273"
)

var dexDotDecredCert = []byte(`-----BEGIN CERTIFICATE-----
MIICeTCCAdqgAwIBAgIQZbivJ9Wrxpx0HHRV9tGn+TAKBggqhkjOPQQDBDA9MSIw
IAYDVQQKExlkY3JkZXggYXV0b2dlbmVyYXRlZCBjZXJ0MRcwFQYDVQQDEw5kZXgu
ZGVjcmVkLm9yZzAeFw0yMDA5MjcxODQwMDZaFw0zMDA5MjYxODQwMDZaMD0xIjAg
BgNVBAoTGWRjcmRleCBhdXRvZ2VuZXJhdGVkIGNlcnQxFzAVBgNVBAMTDmRleC5k
ZWNyZWQub3JnMIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQA3koCrZ4VR/Igiz6z
kOFfhAtfWDWuIot6DIJBdEuXMiPnFZqr8mFAiLP3+ihQNFEc3As7imE4fY5C2KUa
eMed+8IBqgVIlIq1SH99xhceua/UvzG1c+Av9Y2ZEwVgugYJu5d1mbBcomtHTp5n
ctCOOIpQN2KDtUzQqAZQSIrnimzedA+jeTB3MA4GA1UdDwEB/wQEAwICpDAPBgNV
HRMBAf8EBTADAQH/MFQGA1UdEQRNMEuCDmRleC5kZWNyZWQub3Jngglsb2NhbGhv
c3SHBH8AAAGHEAAAAAAAAAAAAAAAAAAAAAGHEP6AAAAAAAAAAAAAAAAAAAGHBAoo
HjIwCgYIKoZIzj0EAwQDgYwAMIGIAkIBlXoes55DGvoOlAVxUW5Ju28Y4ts/ag9k
dDrsQSJuhzbhTcH0iTCq7Sg8bfGuAAP6U492kjqlZepBJUd4WCOyzg4CQgHDOOk5
pO281U39e0XpvQNkT6oJibnCmPVLXuD567Ibt2MfgZet47zGMiOLbQJkv4E8lMv3
wtXxBmKZLaFsxKCm7w==
-----END CERTIFICATE-----
`)

var dexTestSSGenCert = []byte(`-----BEGIN CERTIFICATE-----
MIICojCCAgOgAwIBAgIRANrg1TCbF8AYYNiZjGJIu3cwCgYIKoZIzj0EAwQwODEi
MCAGA1UEChMZZGNyZGV4IGF1dG9nZW5lcmF0ZWQgY2VydDESMBAGA1UEAxMJYmlz
b25leC0yMB4XDTIzMDkyNjIxMTA0MFoXDTMzMDkyNDIxMTA0MFowODEiMCAGA1UE
ChMZZGNyZGV4IGF1dG9nZW5lcmF0ZWQgY2VydDESMBAGA1UEAxMJYmlzb25leC0y
MIGbMBAGByqGSM49AgEGBSuBBAAjA4GGAAQBVcuazn7lwVgl5MTCvVL9Vzp3NPCa
RBzZRUttd0wnHc3DENZ+6C3yGnjvo9gMwRgTV2GwMJYvfWp+qrwJCYyeyyoBGVAA
lpriXQnpEydmYEqtosI8UOz3n3DqrO9c1tp4J8qVdWTNcRHZBeGhLJvLMtQ7u6R0
51m1Q0rIqOFo8V1jJRmjgaowgacwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQF
MAMBAf8wHQYDVR0OBBYEFApbc2a7ufcJcmtZlODB0qfJRUZOMGUGA1UdEQReMFyC
CWJpc29uZXgtMoIJbG9jYWxob3N0gg5iaXNvbi5leGNoYW5nZYcEfwAAAYcQAAAA
AAAAAAAAAAAAAAAAAYcECoAABYcQ/oAAAAAAAABAAQr//oAABYcEIqpMWTAKBggq
hkjOPQQDBAOBjAAwgYgCQgEsKxMiDr4VUOODXTwDhUVP2qppRXfPTcJOpTojeqlc
x45E7Y25oF8E8PRJCDqjmPoXzd819eFB8sCwnZckS55STAJCAXSr0/Cox2ILNTaY
2Di9VmBqKulREa7bIrV4+xJhSCu94C6xSqs1TB5b4roPzwkFJ059uk1yr+bGoHzr
hW4bgtaK
-----END CERTIFICATE-----
`)

var dexSimSSGenCert = []byte(`-----BEGIN CERTIFICATE-----
MIICpTCCAgagAwIBAgIQZMfxMkSi24xMr4CClCODrzAKBggqhkjOPQQDBDBJMSIw
IAYDVQQKExlkY3JkZXggYXV0b2dlbmVyYXRlZCBjZXJ0MSMwIQYDVQQDExp1YnVu
dHUtcy0xdmNwdS0yZ2ItbG9uMS0wMTAeFw0yMDA2MDgxMjM4MjNaFw0zMDA2MDcx
MjM4MjNaMEkxIjAgBgNVBAoTGWRjcmRleCBhdXRvZ2VuZXJhdGVkIGNlcnQxIzAh
BgNVBAMTGnVidW50dS1zLTF2Y3B1LTJnYi1sb24xLTAxMIGbMBAGByqGSM49AgEG
BSuBBAAjA4GGAAQApXJpVD7si8yxoITESq+xaXWtEpsCWU7X+8isRDj1cFfH53K6
/XNvn3G+Yq0L22Q8pMozGukA7KuCQAAL0xnuo10AecWBN0Zo2BLHvpwKkmAs71C+
5BITJksqFxvjwyMKbo3L/5x8S/JmAWrZoepBLfQ7HcoPqLAcg0XoIgJjOyFZgc+j
gYwwgYkwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wZgYDVR0RBF8w
XYIadWJ1bnR1LXMtMXZjcHUtMmdiLWxvbjEtMDGCCWxvY2FsaG9zdIcEfwAAAYcQ
AAAAAAAAAAAAAAAAAAAAAYcEsj5QQYcEChAABYcQ/oAAAAAAAAAYPqf//vUPXDAK
BggqhkjOPQQDBAOBjAAwgYgCQgFMEhyTXnT8phDJAnzLbYRktg7rTAbTuQRDp1PE
jf6b2Df4DkSX7JPXvVi3NeBru+mnrOkHBUMqZd0m036aC4q/ZAJCASa+olu4Isx7
8JE3XB6kGr+s48eIFPtmq1D0gOvRr3yMHrhJe3XDNqvppcHihG0qNb0gyaiX18Cv
vF8Ti1x2vTkD
-----END CERTIFICATE-----
`)

var CertStore = map[string][]byte{
	decredDEXServerMainnet: dexDotDecredCert,
	decredDEXServerTestnet: dexTestSSGenCert,
	decredDEXServerSimnet:  dexSimSSGenCert,
}
