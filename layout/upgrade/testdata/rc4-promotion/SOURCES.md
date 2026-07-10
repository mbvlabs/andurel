# RC.4 promotion candidate source

The RC.4 promotion contract was captured before an RC.4 tag existed. Its source is candidate commit `105ce56ddd54b63baea388e386052733108a0db6`, the exact `HEAD` inspected while implementing the release-blocker remediation.

`internal/server/server.go.golden` is a focused managed-file representative whose body matches the candidate template and whose generated banner is rehydrated as `v1.0.0-rc.4`. Its SHA-256 is `530cb5e10317ad33e0ecc6f09f096fd70ecf1c9c95b78b296b8c83809350afee`. The promotion tests render the stable target and verify that only this exact banner difference is accepted. The candidate commit must be audited again before creating the RC.4 tag.
