## Acquia Hosting Provider Integration

ddev provides integration with the [Acquia Cloud Platform](https://www.acquia.com/choosing-right-acquia-cloud-platform), which allows Acquia users to quickly download and provision a project from Acquia in a local ddev-managed environment.

ddev's Acquia integration pulls database and files from an existing project into your local system so you can develop locally.

### Acquia Quickstart

1. Get your Acquia API token from your Account Settings->API Tokens.
2. Make sure your ssh key is authorized on your Acquia account at Account Settings->SSH Keys
3. `ddev auth ssh` (this typically needs only be done once per ddev session, not every pull.)
4. Add / update the web_environment section in ~/.ddev/global_config.yaml with the API keys:

   ```yaml
   web_environment:
   - ACQUIA_API_KEY=xxxxxxxx
   - ACQUIA_API_SECRET=xxxxx
   ```

5. Copy .ddev/providers/acquia.yaml.example to .ddev/providers/acquia.yaml.
6. Update the project_id corresponding to the environment you want to work with.
   - If have acli install, you can use the following command: `acli remote:aliases:list`
   - Or, on the Acquia Cloud Platform navigate to the environments page, click on the header and look for the "SSH URL" line. Eg. `project1.dev@cool-projects.acquia-sites.com` would have a project ID of `project1.dev`
7. Your project must include drush; `ddev composer require drush/drush` if it isn't there already.
8. `ddev restart`
9. Use `ddev pull acquia` to pull the project database and files.
10. Optionally use `ddev push acquia` to push local files and database to Aquia. Note that `ddev push` is a command that can potentially damage your production site, so this is not recommended. If you want to enable push, you must set the environment variable DDEV_ALLOW_PUSH=true in your .ddev/config.yaml or ~/.ddev/global_config.yaml.

### Usage

`ddev pull acquia` will connect to the Acquia Cloud Platform to download database and files. To skip downloading and importing either file or database assets, use the `--skip-files` and `--skip-db` flags.
