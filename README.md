# YCGuestAgent

YCGuestAgent is a binary executable, running as a service, allowing to reset the user's password on demand.

## Description

Password reset processed by the following flow:

1. A pair of [RSA](https://en.wikipedia.org/wiki/RSA_(cryptosystem)) keys are generated on a user-side (by browser or PowerShell module)
2. YCGuestAgent does the following steps on Virtual Machine:

   * receives and verifies user's request;
   * generates new password;
   * creates a new user with admin permissions if given user does not exist;
   * sets a password to the given Virtual Machine user;
   * encrypts the password by given encryption key;
   * transmits the encrypted password to COM4 port

3. Encrypted password if fetched by Console or PowerShell module
4. Password is decrypted by private key on a user-side and displayed in Console or PowerShell

# YCGuestAgentUpdater

YCGuestAgentUpdater is a binary executable, running as a service, performing updating YCGuestAgent.

## Description

This agent regularly checks if a new version of YCGuestAgent is available.

Then, it downloads a new version and tries to install it and run it.

If the new version cannot start, the update agent fallbacks YCGuestAgent to the previous working version.

