## Platform.sh Hosting Provider Integration

ddev provides integration with the [Platform.sh Website Management Platform](https://platform.sh/), which allows Platform.sh users to quickly download and provision a project from Platform.sh in a local ddev-managed environment.

ddev's Platform.sh integration pulls database and files from an existing Platform.sh site/environment into your local system so you can develop locally.

### Platform.sh Quickstart

1. Check out the site from platform.sh and then configure it with `ddev config`. You'll want to use `ddev start` and make sure the basic functionality is working.
2. Obtain and configure an API token.
   a. Login to the Platform.sh Dashboard and go to Account->API Tokens to create an API token for ddev to use.
   b. Add the API token to the `web_environment` section in your global ddev configuration at ~/.ddev/global_config.yaml:

   ```yaml
   web_environment:
   - PLATFORMSH_CLI_TOKEN=abcdeyourtoken`
   ```

3. `ddev restart`
4. Obtain your project id with `ddev exec platform`. The platform tool should show you all the information about your account and project.
5. In your project's .ddev/providers directory, copy platform.yaml.example to platform.yaml and edit the `project_id` and `environment_name`.
6. Run `ddev pull platform`. After you agree to the prompt, the current upstream database and files will be downloaded.
7. Optionally use `ddev push platform` to push local files and database to Platform.sh. Note that `ddev push` is a command that can potentially damage your production site, so this is not recommended.

**Warning for Apple M1 Users:** There is currently a bug in the linux `platform` command (or its dependencies) which causes it to SEGFAULT in normal use. As a result, you can't run `ddev pull platform` on an M1 machine at this time.

### Usage

`ddev pull platform` will connect to Platform.sh to download database and files. To skip downloading and importing either file or database assets, use the `--skip-files` and `--skip-db` flags.
