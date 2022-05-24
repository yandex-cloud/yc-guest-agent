# YCGuestAgent

YCGuestAgent is a binary executable, running as a service, allowing to reset user password on demand.

## Description

Password reset processed by the following flow:

1. A pair of [RSA](https://en.wikipedia.org/wiki/RSA_(cryptosystem)) keys are generated on a user-side (by browser or PowerShell function) 
1. YCGuestAgent does the following steps on Virtual Machine:

    * receives and verifies user request;
    * generates new password;
    * creates a new user with admin permissions if given user doesn't exist
    * sets password to given Virtual Machine user;
    * encrypts password by given encryption key;
    * transmits encrypted password to user by HTTPS-connection

1. Password is decrypted by private key on a user-side and displayed in Console or PowerShell


# YCGuestAgentUpdater

YCGuestAgentUpdater is a binary executable, running as a service, perfoming updating YCGuestAgent

## Description

This agent regularly checks if a new version of YCGuestAgent is available.

It downloads new version and tries to update.

If new version is unable to start in fallbacks to previous working version. 
