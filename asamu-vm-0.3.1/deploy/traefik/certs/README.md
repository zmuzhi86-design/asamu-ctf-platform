# TLS certificate directory

Before enabling the `proxy` profile, place the deployment certificate chain in
`tls.crt` and its private key in `tls.key`. Keep the private key permission at
`0600`; neither file is copied into an application image.
