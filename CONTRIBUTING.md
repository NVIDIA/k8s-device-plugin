# Contribute to the NVIDIA Kubernetes Device Plugin

Want to hack on the NVIDIA Kubernetes Device plugin Project? Awesome!
We only require you to sign your work, the below section describes this!

All contributions must adhere to the [Code of Conduct](CODE_OF_CONDUCT.md).

## Open an issue first

Before beginning implementation or opening a pull request for a significant change,
first open an issue describing the problem or proposal. A significant change includes,
but is not limited to, architectural changes, new features, breaking changes to API or
behavior, and non-trivial bug fixes.

Waiting for a maintainer to acknowledge the issue before you start writing code avoids
work that turns out to conflict with the direction of the project. See
[GOVERNANCE.md](GOVERNANCE.md) for how these decisions are made.

Two exceptions:

- Trivial changes — typos, formatting, broken links — can go straight to a pull request.
- Security vulnerabilities must never start with a public issue. Report them through
  the process in [SECURITY.md](SECURITY.md) instead.

Reference the issue in your pull request description with `Closes #1234` so that merging
the pull request closes it.

## Review process

All changes land through a pull request against `main`; nobody pushes to `main` directly.
Opening a pull request runs the CI suite — lint, unit tests, `helm` tests, CodeQL
analysis, image builds, and end-to-end tests — alongside a Developer Certificate of
Origin check on your commits. All of these must pass before a pull request can merge.

A pull request also needs an approving review from a maintainer. Maintainers make the
final call on what is accepted; see [GOVERNANCE.md](GOVERNANCE.md) for the decision
making model.

If your pull request has gone quiet, comment on it to ask for attention. That is welcome
and effective — a stalled review is usually an oversight rather than a rejection.

## Sign your work

The sign-off is a simple line at the end of the explanation for the patch. Your
signature certifies that you wrote the patch or otherwise have the right to pass
it on as an open-source patch. The rules are pretty simple: if you can certify
the below (from [developercertificate.org](http://developercertificate.org/)):

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.
1 Letterman Drive
Suite D4700
San Francisco, CA, 94129

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.

Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```

Then you just add a line to every git commit message:

    Signed-off-by: Joe Smith <joe.smith@email.com>

Use your real name (sorry, no pseudonyms or anonymous contributions.)

If you set your `user.name` and `user.email` git configs, you can sign your
commit automatically with `git commit -s`.

