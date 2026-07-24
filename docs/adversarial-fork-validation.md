# Adversarial fork validation fixture

These inert files exercise the trusted SonarCloud pull-request boundary. Each executable surface fails with exit code 97 if invoked. Successful acceptance requires the protected analyzer to obtain only verified regular Go blobs through the Git Data API and to leave these fork-controlled inputs unmaterialized and unexecuted:

- `testdata/adversarial-repository/.github/workflows/adversarial-fork-fixture.yml`
- `.github/actions/adversarial-fork/action.yml`
- `sonar-project.properties`
- `package.json` dependency hooks
- `Dockerfile.adversarial` and `compose.adversarial.yml`
- `scripts/adversarial-fork-must-not-run.sh`
- `tools/adversarial-fork-binary`

The fixture contains no credential, destructive command, network call, or persistent side effect.
