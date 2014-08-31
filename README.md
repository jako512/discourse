Discourse Juju Charm
===============

This charm installs the [Discourse](http://discourse.org) application, a modern discussion forum.

# Overview

This charm deploys a docker container that contains the discourse software.

1 GB RAM is required (with swap), 2 GB is recommended.

Also remember to check the [icon guidelines](https://juju.ubuntu.com/docs/authors-charm-icon.html) so that your charm looks good in the Juju GUI.

# Install

After you've successfully bootstrapped an environment, run the following:

    juju deploy cs:/~natefinch/discourse
    juju expose discourse









