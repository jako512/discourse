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

Note: before you begin, you should set up email sending service somewhere -
mandrill or mailgun or any other number of providers offer free email service
with a high enough monthly quota for all but very large sites.  You need the
email service to authenticate the administrative users, and you need the info
from the mail service to set as configuration on the charm.

After you've successfully bootstrapped a Juju environment, run the following:

    juju deploy cs:~natefinch/discourse --config=cfg.yml --constraints mem=2G
    juju expose discourse

Discourse recommends 2GB of RAM for a standard installation.  You can use less,
but if so, you should set UNICORN_WORKERS to 2.  See the above link to the
install guide for more details.  If deploying to AWS, note that an m1.small has
1.75GB of RAM, which is probably fine and setting the constraint to 2GB will get
you a much bigger and more expensive machine.

For quickest deploy, it is strongly suggested that you give the charm
configuration during bootstrap, this prevents additional wait time from setting
values afterward, since applying config changes can take a few minutes.

A valid configuration file looks something like this:

    discourse:
      DISCOURSE_HOSTNAME: discuss.example.com
      DISCOURSE_DEVELOPER_EMAILS: foo@example.com,bar@example.com
      DISCOURSE_SMTP_ADDRESS: smtp.mailservice.com
      DISCOURSE_SMTP_PORT: 587
      DISCOURSE_SMTP_USER_NAME: postmaster@example.com
      DISCOURSE_SMTP_PASSWORD: supersecretpassword
      UNICORN_WORKERS: 3

(change "discourse" to match the name of your service, which default to
"discourse") Note that the spaces in front of the capitalized values are
required.

It is normal and expected for the install process to take quite a long
time.  On an m1.small on AWS, it takes approximately 21 minutes from `juju
deploy` to being able to bring up Discourse in a browser.  On a 2GB Digital
Ocean droplet it takes approximately 7 minutes.  Obviously this time may vary a
lot depending on where you're deploying.

You can always update the configuration after deployment using `juju set`, which
will rebuild your discourse setup and take a few minutes, but will not lose any
data.

    juju set discourse --config=cfg.yml

# Debugging

If you experience any problems and want to modify the hook code after it's
deployed, you'll need to regenerate the hooks executable file.  To do that,
you'll need to install Go on the host machine.  To make this easier, there is a
debugsetup.sh script in the hooks directly.  Simply **source** that file (don't
just run it - it sets an environment variable, too, which only works if you
source it).

    source ./debugsetup.sh

And now you can modify the hooks.go file and rebuild the hooks executable by
running

    go build hooks.go

# Testing

The test file hooks.test under the /test directory is a Go test binary.  To
rebuild it, source debugsetup.sh as described under debugging, and then, in the
hooks directory, run

    go test -c

This will create the hooks.test file, which can then be moved to the /test
directory.  If you want to run the tests manually, just run `go test` from the
hooks directory.
