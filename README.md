Discourse Juju Charm
===============

This charm installs the [Discourse](http://discourse.org) application, a modern discussion forum.

# Overview

This charm deploys the Discourse standalone docker container as per the [Install Guide](https://github.com/discourse/discourse/blob/master/docs/INSTALL-digital-ocean.md).

Note that because this charm deploys a docker container, it cannot be deployed to a container created by Juju.  It must be deployed to the base machine.

# Install

After you've successfully bootstrapped an environment, run the following:

    juju deploy cs:/~natefinch/discourse
    juju expose discourse


    









