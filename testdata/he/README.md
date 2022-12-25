# Cert-Manager ACME DNS01 Webhook Solver for HE DNS

## testdata directory

Copy the example Secret files, replacing `$HE_USERNAME` and `$HE_PASSWORD` with your
actual HE credentials:

```bash
export HE_USERNAME="<your HE username>"
export HE_PASSWORD="<your HE password>"
sed "s/%%HE_USERNAME%%/${HE_USERNAME}/; s/%%HE_PASSWORD%%/${HE_PASSWORD}/" testdata/he/secret.yaml.example > testdata/he/secret.yaml
```
