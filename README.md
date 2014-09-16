Discourse Juju Charm
===============

This charm installs the [Discourse](http://discourse.org) application, a modern
discussion forum.

# Overview

This charm deploys the Discourse standalone docker container as per the [Install
Guide](https://github.com/discourse/discourse/blob/master/docs/INSTALL-digital-
ocean.md).

Note that because this charm deploys a docker container, it cannot be deployed
to a container created by Juju.  It must be deployed to the base machine.

# Install

After you've successfully bootstrapped an environment, run the following:

    juju deploy cs:/~natefinch/discourse
    juju expose discourse

# Debugging

If you experience problems and want to add debugging output to the script,
you'll need to regenerate the hooks executable file.  To do that, you'll need to
install go on the host machine.  To make this easier, there is a debugSetup.sh
script in the hooks directly.  Simple **source** that file (don't just run it -
it sets an environment variable, too).

	source ./debugSetup.sh

And now you can modify the hooks.go file and rebuild the hooks executable by
running

    go build hooks.go


    
