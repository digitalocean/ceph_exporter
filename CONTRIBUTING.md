Contributing
============

Please use this guide before making any contributions to this repository.

Preliminary
-----------

* All code **must** be [`gofmt`](https://golang.org/cmd/gofmt/)'d, [`golint`](https://github.com/golang/lint)'d and [`go vet`](https://golang.org/cmd/vet/)'d before being committed.
* Code **should** have test coverage to ensure its correctness.

PRs
---

**Commits**

Keep individual commits descriptive. Prefix them with the collector name and a
colon. Anyone viewing the git history should be able to determine from those
first 80 characters, the body of the commit. Feel free to expand further on
the commit but keep the first 80 characters on point.

Good Commit:

```
monitor: expose metrics for clock skew
  - scrape monitor's skew value from ceph's status
```

Bad Commit:

```
new monitor metrics
```

Use your own discretion when deciding whether or not to squash multiple commits
in a PR to a single commit.  However, each commit should contain a single,
logical unit of change, and a descriptive message.

Resources
---------

* [Effective Go](https://golang.org/doc/effective_go.html)
* [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
* [How to Write Go Code](https://golang.org/doc/code.html)
* [Twelve Go Best Practices](https://talks.golang.org/2013/bestpractices.slide)
