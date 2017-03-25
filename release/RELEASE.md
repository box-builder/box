## Box Version @@VERSION@@

Box is a small utility for the building of docker images. Through use of mruby,
we provide additional flexibility over the vanilla `docker build` command by
adding control structures and basic predicates. We also add new verbs that
allow new actions, such as flattening and tagging images.

Some features that differentiate it from `docker build`:

* Unique general features:
  * mruby syntax
  * Filtering of keywords to secure builds
  * Shell mode
* In the build plan itself:
  * Layer editing and composition
  * Tagging
  * Flattening
  * Debug mode (drop to a shell in the middle of a plan run and inspect your container)
  * Ruby block methods for `user` ([with\_user](https://box-builder.github.io/user-guide/verbs/#with95user)) and `workdir` ([inside](https://box-builder.github.io/user-guide/verbs/#inside)) allow
    you to scope `copy` and `run` operations for a more obvious build plan.


This is the @@VERSION@@ release of Box, the mruby-inspired advanced docker
builder. If you're new to Box, you can read the documentation [here](https://box-builder.github.io/box/).

The changes included in this version of Box are:

