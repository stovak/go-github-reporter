# Github Reporter

Primary Usage:

```
  task build
 ./ghrp repos:find --debug ".circleci/config.yml" [--debug] [--group "@pantheon-systems/developer-experience"]
```

Returns a list of repositories where the `CODEOWNERS` file contains `@pantheon-systems/developer-experience` and that have the file `.circleci/config.yml` in the default branch.


Result:
```
✅ repo1 => .circleci/config.yml
✅ repo2 => .circleci/config.yml
✅ go-repo-3 => .circleci/config.yml
```

