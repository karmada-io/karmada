## [v0.2.7](https://github.com/adhocore/gronx/releases/tag/v0.2.7) (2022-06-28)

### Miscellaneous
- **Workflow**: Run tests on 1.18x (Jitendra)
- Tests for go v1.17.x, add codecov (Jitendra)


## [v0.2.6](https://github.com/adhocore/gronx/releases/tag/v0.2.6) (2021-10-14)

### Miscellaneous
- Fix 'with' languages (Jitendra Adhikari) [_a813b55_](https://github.com/adhocore/gronx/commit/a813b55)
- Init/setup github codeql (Jitendra Adhikari) [_fe2aa5a_](https://github.com/adhocore/gronx/commit/fe2aa5a)


## [v0.2.5](https://github.com/adhocore/gronx/releases/tag/v0.2.5) (2021-07-25)

### Bug Fixes
- **Tasker**: The clause should be using OR (Jitendra Adhikari) [_b813b85_](https://github.com/adhocore/gronx/commit/b813b85)


## [v0.2.4](https://github.com/adhocore/gronx/releases/tag/v0.2.4) (2021-05-05)

### Features
- **Pkg.tasker**: Capture cmd output in tasker logger, error in stderr (Jitendra Adhikari) [_0da0aae_](https://github.com/adhocore/gronx/commit/0da0aae)

### Internal Refactors
- **Cmd.tasker**: Taskify is now method of tasker (Jitendra Adhikari) [_8b1373b_](https://github.com/adhocore/gronx/commit/8b1373b)


## [v0.2.3](https://github.com/adhocore/gronx/releases/tag/v0.2.3) (2021-05-04)

### Bug Fixes
- **Pkg.tasker**: Sleep 100ms so abort can be bailed asap, remove dup msg (Jitendra Adhikari) [_d868920_](https://github.com/adhocore/gronx/commit/d868920)

### Miscellaneous
- Allow leeway period at the end (Jitendra Adhikari) [_5ebf923_](https://github.com/adhocore/gronx/commit/5ebf923)


## [v0.2.2](https://github.com/adhocore/gronx/releases/tag/v0.2.2) (2021-05-03)

### Bug Fixes
- **Pkg.tasker**: DoRun checks if timed out before run (Jitendra Adhikari) [_f27a657_](https://github.com/adhocore/gronx/commit/f27a657)

### Internal Refactors
- **Pkg.tasker**: Use dateFormat var, update final tick phrase (Jitendra Adhikari) [_fad0271_](https://github.com/adhocore/gronx/commit/fad0271)


## [v0.2.1](https://github.com/adhocore/gronx/releases/tag/v0.2.1) (2021-05-02)

### Bug Fixes
- **Pkg.tasker**: Deprecate sleep dur if next tick timeout (Jitendra Adhikari) [_3de45a1_](https://github.com/adhocore/gronx/commit/3de45a1)


## [v0.2.0](https://github.com/adhocore/gronx/releases/tag/v0.2.0) (2021-05-02)

### Features
- **Cmd.tasker**: Add tasker for standalone usage as task daemon (Jitendra Adhikari) [_0d99409_](https://github.com/adhocore/gronx/commit/0d99409)
- **Pkg.tasker**: Add parser for tasker pkg (Jitendra Adhikari) [_e7f1811_](https://github.com/adhocore/gronx/commit/e7f1811)
- **Pkg.tasker**: Add tasker pkg (Jitendra Adhikari) [_a57b1c4_](https://github.com/adhocore/gronx/commit/a57b1c4)

### Bug Fixes
- **Pkg.tasker**: Use log.New() instead (Jitendra Adhikari) [_0cf2c07_](https://github.com/adhocore/gronx/commit/0cf2c07)
- **Validator**: This check is not really required (Jitendra Adhikari) [_c3d75e3_](https://github.com/adhocore/gronx/commit/c3d75e3)

### Internal Refactors
- **Gronx**: Add public methods for internal usage, expose spaceRe (Jitendra Adhikari) [_94eb20b_](https://github.com/adhocore/gronx/commit/94eb20b)

### Miscellaneous
- **Pkg.tasker**: Use file perms as octal (Jitendra Adhikari) [_83f258d_](https://github.com/adhocore/gronx/commit/83f258d)
- **Workflow**: Include all tests in action (Jitendra Adhikari) [_7328cbf_](https://github.com/adhocore/gronx/commit/7328cbf)

### Documentations
- Add task mangager and tasker docs/usages (Jitendra Adhikari) [_e77aa5f_](https://github.com/adhocore/gronx/commit/e77aa5f)


## [v0.1.4](https://github.com/adhocore/gronx/releases/tag/v0.1.4) (2021-04-25)

### Miscellaneous
- **Mod**: 1.13 is okay too (Jitendra Adhikari) [_6c328e7_](https://github.com/adhocore/gronx/commit/6c328e7)
- Try go 1.13.x (Jitendra Adhikari) [_b017ec4_](https://github.com/adhocore/gronx/commit/b017ec4)

### Documentations
- Practical usage (Jitendra Adhikari) [_9572e61_](https://github.com/adhocore/gronx/commit/9572e61)


## [v0.1.3](https://github.com/adhocore/gronx/releases/tag/v0.1.3) (2021-04-22)

### Internal Refactors
- **Checker**: Preserve error, for pos 2 & 4 bail only on due or err (Jitendra Adhikari) [_39a9cd5_](https://github.com/adhocore/gronx/commit/39a9cd5)
- **Validator**: Do not discard error from strconv (Jitendra Adhikari) [_3b0f444_](https://github.com/adhocore/gronx/commit/3b0f444)


## [v0.1.2](https://github.com/adhocore/gronx/releases/tag/v0.1.2) (2021-04-21)

### Features
- Add IsValid() (Jitendra Adhikari) [_150687b_](https://github.com/adhocore/gronx/commit/150687b)

### Documentations
- IsValid usage (Jitendra Adhikari) [_b747116_](https://github.com/adhocore/gronx/commit/b747116)


## [v0.1.1](https://github.com/adhocore/gronx/releases/tag/v0.1.1) (2021-04-21)

### Features
- Add main gronx api (Jitendra Adhikari) [_1b3b108_](https://github.com/adhocore/gronx/commit/1b3b108)
- Add cron segment checker (Jitendra Adhikari) [_a56be7c_](https://github.com/adhocore/gronx/commit/a56be7c)
- Add validator (Jitendra Adhikari) [_455a024_](https://github.com/adhocore/gronx/commit/455a024)

### Miscellaneous
- **Workflow**: Update actions (Jitendra Adhikari) [_8b54cc3_](https://github.com/adhocore/gronx/commit/8b54cc3)
- Init module (Jitendra Adhikari) [_bada37d_](https://github.com/adhocore/gronx/commit/bada37d)
- Add license (Jitendra Adhikari) [_5f20b96_](https://github.com/adhocore/gronx/commit/5f20b96)
- **Gh**: Add meta files (Jitendra Adhikari) [_35a1310_](https://github.com/adhocore/gronx/commit/35a1310)
- **Workflow**: Add lint/test actions (Jitendra Adhikari) [_884d5cb_](https://github.com/adhocore/gronx/commit/884d5cb)
- Add editorconfig (Jitendra Adhikari) [_8b75494_](https://github.com/adhocore/gronx/commit/8b75494)

### Documentations
- On cron expressions (Jitendra Adhikari) [_547fd72_](https://github.com/adhocore/gronx/commit/547fd72)
- Add readme (Jitendra Adhikari) [_3955e88_](https://github.com/adhocore/gronx/commit/3955e88)
