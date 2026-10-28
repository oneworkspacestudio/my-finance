# Acceptance evidence

Work in progress. The first four Go tests pass: authenticated backup roundtrip and
tamper/wrong-key/truncation rejection; refusing server-secret archives; consent
binding/expiry/replay; and native route rebasing. Go vet passes.

Native Windows runtime, browser, household ACL, manual-import and clean-restore
acceptance are pending. No provider sandbox or live bank tests have been run.
No release or installation on either owner machine has been completed yet.

Final FIN-01–FIN-22 matrix will be written after verification. Untested criteria
must remain NOT TESTED, and deferred criteria must remain explicitly deferred.
