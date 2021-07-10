## Pantheon Hosting Provider Integration

ddev provides configurable integration with the [Pantheon Website Management Platform](https://pantheon.io/), which allows Pantheon users to quickly download and provision a project from Pantheon in a local ddev-managed environment.

ddev's Pantheon integration pulls an existing backup from an existing Pantheon site/environment into your local system. Of course that means you must already have a Pantheon site with a backup in order to use it.

### Pantheon Quickstart

If you have ddev installed, and have an active Pantheon account with an active site, you can follow this guide to spin up a Pantheon project locally.

1. Get your Pantheon.io machine token:
   a. Login to your Pantheon Dashboard, and [Generate a Machine Token](https://pantheon.io/docs/machine-tokens/) for ddev to use.
   b. Add the API token to the `web_environment` section in your global ddev configuration at ~/.ddev/global_config.yaml

   ```
   web_environment:
   - TERMINUS_MACHINE_TOKEN=abcdeyourtoken`
   ```

2. Choose a Pantheon site and environment you want to use with ddev. You can usually use the site name, but in some environments you may need the site ID, which is the long 3rd component of your site dashboard URL. So if the site dashboard is at <https://dashboard.pantheon.io/sites/009a2cda-2c22-4eee-8f9d-96f017321555#dev/>, the site ID is 009a2cda-2c22-4eee-8f9d-96f017321555.

3. On the pantheon dashboard for the site, make sure that at least one backup has been created. (When you need to refresh what you pull, create a new backup.)

4. Make sure your public ssh key is configured in Pantheon (Account->SSH Keys)

5. Check out the project codebase from Pantheon. Enable the "Git Connection Mode" and use `git clone` to check out the code locally.

6. Configure the local checkout for ddev using `ddev config`

7. If using Drupal 8+, verify that drush is installed in your project, `ddev composer require drush/drush`. If using Drupal 6 or 7, drush8 is already provided in the web container's /usr/local/bin/drush, so you can skip this step.

8. In your project's .ddev/providers directory, copy pantheon.yaml.example to pantheon.yaml and edit the `project` environment variable under `environment_variables`. It will be in the format `<projectname>.<environment>`, for example `yourprojectname.dev` or (in cases of ambiguity) `<project_uuid>.<environment>`, for example `009a2cda-2c22-4eee-8f9d-96f017321555.dev`.

9. `ddev restart`

10. Run `ddev pull pantheon`. The ddev environment  download the Pantheon database and files, and import the database and files into the ddev environment. You should now be able to access the project locally.

11. Optionally use `ddev push pantheon` to push local files and database to Pantheon. Note that `ddev push` is a command that can potentially damage your production site, so this is not recommended. If you want to enable push, you must set the environment variable DDEV_ALLOW_PUSH=true in your .ddev/config.yaml or ~/.ddev/global_config.yaml.
