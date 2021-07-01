package ddevapp

// DDevComposeTemplate is used to create the main docker-compose file
// file for a ddev project.
const DDevComposeTemplate = `version: '{{ .ComposeVersion }}'
{{ .DdevGenerated }}
services:
{{if not .OmitDB }}
  db:
    container_name: {{ .Plugin }}-${DDEV_SITENAME}-db
    build:
      context: '{{ .DBBuildContext }}'
      dockerfile: '{{ .DBBuildDockerfile }}'
      args:
        BASE_IMAGE: $DDEV_DBIMAGE
        username: '{{ .Username }}'
        uid: '{{ .UID }}'
        gid: '{{ .GID }}'
    image: ${DDEV_DBIMAGE}-${DDEV_SITENAME}-built
    stop_grace_period: 60s
    volumes:
      - type: "volume"
        source: mariadb-database
        target: "/var/lib/mysql"
        volume:
          nocopy: true
      - type: "bind"
        source: "."
        target: "/mnt/ddev_config"
      - ddev-global-cache:/mnt/ddev-global-cache
    restart: "{{ if .AutoRestartContainers }}always{{ else }}no{{ end }}"
    user: "$DDEV_UID:$DDEV_GID"
    hostname: {{ .Name }}-db
    ports:
      - "{{ .DockerIP }}:$DDEV_HOST_DB_PORT:3306"
    labels:
      com.ddev.site-name: ${DDEV_SITENAME}
      com.ddev.platform: {{ .Plugin }}
      com.ddev.app-type: {{ .AppType }}
      com.ddev.approot: $DDEV_APPROOT
    environment:
      - COLUMNS
      - DDEV_HOSTNAME
      - DDEV_PHP_VERSION
      - DDEV_PRIMARY_URL
      - DDEV_PROJECT
      - DDEV_PROJECT_TYPE
      - DDEV_ROUTER_HTTP_PORT
      - DDEV_ROUTER_HTTPS_PORT
      - DDEV_SITENAME
      - DDEV_TLD
      - DOCKER_IP={{ .DockerIP }}
      - HOST_DOCKER_INTERNAL_IP={{ .HostDockerInternalIP }}
      - IS_DDEV_PROJECT=true
      - LINES
      - TZ={{ .Timezone }}
    command: "$DDEV_MARIADB_LOCAL_COMMAND"
    healthcheck:
      interval: 1s
      retries: 120
      start_period: 120s
      timeout: 120s
{{end}}
  web:
    container_name: {{ .Plugin }}-${DDEV_SITENAME}-web
    build:
      context: '{{ .WebBuildContext }}'
      dockerfile: '{{ .WebBuildDockerfile }}'
      args:
        BASE_IMAGE: $DDEV_WEBIMAGE
        username: '{{ .Username }}'
        uid: '{{ .UID }}'
        gid: '{{ .GID }}'
    image: ${DDEV_WEBIMAGE}-${DDEV_SITENAME}-built
    cap_add:
      - SYS_PTRACE
    volumes:
      {{ if not .NoProjectMount }}
      - type: {{ .MountType }}
        source: {{ .WebMount }}
        target: /var/www/html
        {{ if eq .MountType "volume" }}
        volume:
          nocopy: true
        {{ else }}
        consistency: cached
        {{ end }}
      {{ end }}
      - ".:/mnt/ddev_config:ro"
      - "./nginx_full:/etc/nginx/sites-enabled:ro"
      - "./apache:/etc/apache2/sites-enabled:ro"
      - ddev-global-cache:/mnt/ddev-global-cache
      {{ if not .OmitSSHAgent }}
      - ddev-ssh-agent_socket_dir:/home/.ssh-agent
      {{ end }}

    restart: "{{ if .AutoRestartContainers }}always{{ else }}no{{ end }}"
    user: "$DDEV_UID:$DDEV_GID"
    hostname: {{ .Name }}-web
    {{if not .OmitDB }}
    links:
      - db:db
    {{end}}
    # ports is list of exposed *container* ports
    ports:
      - "{{ .DockerIP }}:$DDEV_HOST_WEBSERVER_PORT:80"
      - "{{ .DockerIP }}:$DDEV_HOST_HTTPS_PORT:443"
    environment:
      - COLUMNS
      - DOCROOT=${DDEV_DOCROOT}
      - DDEV_DOCROOT
      - DDEV_HOSTNAME
      - DDEV_PHP_VERSION
      - DDEV_PRIMARY_URL
      - DDEV_PROJECT
      - DDEV_PROJECT_TYPE
      - DDEV_ROUTER_HTTP_PORT
      - DDEV_ROUTER_HTTPS_PORT
      - DDEV_SITENAME
      - DDEV_TLD
      - DDEV_FILES_DIR
      - DDEV_WEBSERVER_TYPE
      - DDEV_XDEBUG_ENABLED
      - DEPLOY_NAME=local
{{ if not .DisableSettingsManagement }}
      - DRUSH_OPTIONS_URI=$DDEV_PRIMARY_URL
{{ end }}
      - DRUSH_ALLOW_XDEBUG=1
      - DOCKER_IP={{ .DockerIP }}
      - HOST_DOCKER_INTERNAL_IP={{ .HostDockerInternalIP }}
      # HTTP_EXPOSE allows for ports accepting HTTP traffic to be accessible from <site>.ddev.site:<port>
      # To expose a container port to a different host port, define the port as hostPort:containerPort
      - HTTP_EXPOSE=${DDEV_ROUTER_HTTP_PORT}:80,${DDEV_MAILHOG_PORT}:{{ .MailhogPort }}
      # You can optionally expose an HTTPS port option for any ports defined in HTTP_EXPOSE.
      # To expose an HTTPS port, define the port as securePort:containerPort.
      - HTTPS_EXPOSE=${DDEV_ROUTER_HTTPS_PORT}:80,${DDEV_MAILHOG_HTTPS_PORT}:{{ .MailhogPort }}
      - IS_DDEV_PROJECT=true
      - LINES
      - SSH_AUTH_SOCK=/home/.ssh-agent/socket
      - TZ={{ .Timezone }}
      - VIRTUAL_HOST=${DDEV_HOSTNAME}
      {{ range $env := .WebEnvironment }}- "{{ $env }}"
      {{ end }}
    labels:
      com.ddev.site-name: ${DDEV_SITENAME}
      com.ddev.platform: {{ .Plugin }}
      com.ddev.app-type: {{ .AppType }}
      com.ddev.approot: $DDEV_APPROOT
{{ if .HostDockerInternalIP }}
    extra_hosts: [ "host.docker.internal:{{ .HostDockerInternalIP }}" ]
{{ end }}
    external_links:
    {{ range $hostname := .Hostnames }}- "ddev-router:{{ $hostname }}"
    {{ end }}
    healthcheck:
      interval: 1s
      retries: 120
      start_period: 120s
      timeout: 120s

{{ if not .OmitDBA }}
  dba:
    container_name: ddev-${DDEV_SITENAME}-dba
    image: $DDEV_DBAIMAGE
    restart: "{{ if .AutoRestartContainers }}always{{ else }}no{{ end }}"
    labels:
      com.ddev.site-name: ${DDEV_SITENAME}
      com.ddev.platform: {{ .Plugin }}
      com.ddev.app-type: {{ .AppType }}
      com.ddev.approot: $DDEV_APPROOT
    links:
      - db:db
    ports:
      - "80"
    hostname: {{ .Name }}-dba
    environment:
      - PMA_USER=root
      - PMA_PASSWORD=root
      - VIRTUAL_HOST=$DDEV_HOSTNAME
      - UPLOAD_LIMIT=1024M
      - TZ={{ .Timezone }}
      # HTTP_EXPOSE allows for ports accepting HTTP traffic to be accessible from <site>.ddev.site:<port>
      - HTTP_EXPOSE=${DDEV_PHPMYADMIN_PORT}:{{ .DBAPort }}
      - HTTPS_EXPOSE=${DDEV_PHPMYADMIN_HTTPS_PORT}:{{ .DBAPort }}
    healthcheck:
      interval: 120s
      timeout: 2s
      retries: 1
{{end}}
networks:
  default:
    name: ddev_default
    external: true
volumes:
  {{if not .OmitDB }}
  mariadb-database:
    name: "${DDEV_SITENAME}-mariadb"
  {{end}}
  {{ if not .OmitSSHAgent }}
  ddev-ssh-agent_socket_dir:
    external: true
  {{ end }}
  ddev-global-cache:
    name: ddev-global-cache

  {{ if and .NFSMountEnabled (not .NoProjectMount) }}
  nfsmount:
    name: ddev-${DDEV_SITENAME}_nfsmount
    driver: local
    driver_opts:
      type: nfs
      o: "addr={{ if .HostDockerInternalIP }}{{ .HostDockerInternalIP }}{{ else }}host.docker.internal{{end}},hard,nolock,rw"
      device: ":{{ .NFSSource }}"
  {{ end }}

  `

// ConfigInstructions is used to add example hooks usage
const ConfigInstructions = `
# Key features of ddev's config.yaml:

# name: <projectname> # Name of the project, automatically provides
#   http://projectname.ddev.site and https://projectname.ddev.site

# type: <projecttype>  # drupal6/7/8, backdrop, typo3, wordpress, php

# docroot: <relative_path> # Relative path to the directory containing index.php.

# php_version: "7.4"  # PHP version to use, "5.6", "7.0", "7.1", "7.2", "7.3", "7.4" "8.0"

# You can explicitly specify the webimage, dbimage, dbaimage lines but this
# is not recommended, as the images are often closely tied to ddev's' behavior,
# so this can break upgrades.

# webimage: <docker_image>  # nginx/php docker image.
# dbimage: <docker_image>  # mariadb docker image.
# dbaimage: <docker_image>

# mariadb_version and mysql_version
# ddev can use many versions of mariadb and mysql
# However these directives are mutually exclusive
# mariadb_version: 10.2
# mysql_version: 8.0

# router_http_port: <port>  # Port to be used for http (defaults to port 80)
# router_https_port: <port> # Port for https (defaults to 443)

# xdebug_enabled: false  # Set to true to enable xdebug and "ddev start" or "ddev restart"
# Note that for most people the commands
# "ddev xdebug" to enable xdebug and "ddev xdebug off" to disable it work better,
# as leaving xdebug enabled all the time is a big performance hit.

# xhprof_enabled: false  # Set to true to enable xhprof and "ddev start" or "ddev restart"
# Note that for most people the commands
# "ddev xhprof" to enable xhprof and "ddev xhprof off" to disable it work better,
# as leaving xhprof enabled all the time is a big performance hit.

# webserver_type: nginx-fpm  # or apache-fpm

# timezone: Europe/Berlin
# This is the timezone used in the containers and by PHP;
# it can be set to any valid timezone,
# see https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
# For example Europe/Dublin or MST7MDT

# composer_version: ""
# if composer_version:"" it will use the current ddev default composer release.
# It can also be set to "1", to get most recent composer v1
# or "2" for most recent composer v2.
# It can be set to any existing specific composer version.
# After first project 'ddev start' this will not be updated until it changes

# additional_hostnames:
#  - somename
#  - someothername
# would provide http and https URLs for "somename.ddev.site"
# and "someothername.ddev.site".

# additional_fqdns:
#  - example.com
#  - sub1.example.com
# would provide http and https URLs for "example.com" and "sub1.example.com"
# Please take care with this because it can cause great confusion.

# upload_dir: custom/upload/dir
# would set the destination path for ddev import-files to custom/upload/dir.

# working_dir:
#   web: /var/www/html
#   db: /home
# would set the default working directory for the web and db services.
# These values specify the destination directory for ddev ssh and the
# directory in which commands passed into ddev exec are run.

# omit_containers: [db, dba, ddev-ssh-agent]
# Currently only these containers are supported. Some containers can also be
# omitted globally in the ~/.ddev/global_config.yaml. Note that if you omit
# the "db" container, several standard features of ddev that access the
# database container will be unusable.

# nfs_mount_enabled: false
# Great performance improvement but requires host configuration first.
# See https://ddev.readthedocs.io/en/stable/users/performance/#using-nfs-to-mount-the-project-into-the-container

# fail_on_hook_fail: False
# Decide whether 'ddev start' should be interrupted by a failing hook

# host_https_port: "59002"
# The host port binding for https can be explicitly specified. It is
# dynamic unless otherwise specified.
# This is not used by most people, most people use the *router* instead
# of the localhost port.

# host_webserver_port: "59001"
# The host port binding for the ddev-webserver can be explicitly specified. It is
# dynamic unless otherwise specified.
# This is not used by most people, most people use the *router* instead
# of the localhost port.

# host_db_port: "59002"
# The host port binding for the ddev-dbserver can be explicitly specified. It is dynamic
# unless explicitly specified.

# phpmyadmin_port: "8036"
# phpmyadmin_https_port: "8037"
# The PHPMyAdmin ports can be changed from the default 8036 and 8037

# mailhog_port: "8025"
# mailhog_https_port: "8026"
# The MailHog ports can be changed from the default 8025 and 8026

# webimage_extra_packages: [php7.4-tidy, php-bcmath]
# Extra Debian packages that are needed in the webimage can be added here

# dbimage_extra_packages: [telnet,netcat]
# Extra Debian packages that are needed in the dbimage can be added here

# use_dns_when_possible: true
# If the host has internet access and the domain configured can
# successfully be looked up, DNS will be used for hostname resolution
# instead of editing /etc/hosts
# Defaults to true

# project_tld: ddev.site
# The top-level domain used for project URLs
# The default "ddev.site" allows DNS lookup via a wildcard
# If you prefer you can change this to "ddev.local" to preserve
# pre-v1.9 behavior.

# ngrok_args: --subdomain mysite --auth username:pass
# Provide extra flags to the "ngrok http" command, see
# https://ngrok.com/docs#http or run "ngrok http -h"

# disable_settings_management: false
# If true, ddev will not create CMS-specific settings files like
# Drupal's settings.php/settings.ddev.php or TYPO3's AdditionalConfiguration.php
# In this case the user must provide all such settings.

# You can inject environment variables into the web container with:
# web_environment:
# - SOMEENV=somevalue
# - SOMEOTHERENV=someothervalue

# no_project_mount: false
# (Experimental) If true, ddev will not mount the project into the web container;
# the user is responsible for mounting it manually or via a script.
# This is to enable experimentation with alternate file mounting strategies.
# For advanced users only!

# Many ddev commands can be extended to run tasks before or after the
# ddev command is executed, for example "post-start", "post-import-db",
# "pre-composer", "post-composer"
# See https://ddev.readthedocs.io/en/stable/users/extending-commands/ for more
# information on the commands that can be extended and the tasks you can define
# for them. Example:
#hooks:
`

// SequelproTemplate is the template for Sequelpro config.
var SequelproTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>ContentFilters</key>
    <dict/>
    <key>auto_connect</key>
    <true/>
    <key>data</key>
    <dict>
        <key>connection</key>
        <dict>
            <key>database</key>
            <string>%s</string>
            <key>host</key>
            <string>%s</string>
            <key>name</key>
            <string>drud/%s</string>
            <key>password</key>
            <string>%s</string>
            <key>port</key>
            <integer>%s</integer>
            <key>rdbms_type</key>
            <string>mysql</string>
            <key>sslCACertFileLocation</key>
            <string></string>
            <key>sslCACertFileLocationEnabled</key>
            <integer>0</integer>
            <key>sslCertificateFileLocation</key>
            <string></string>
            <key>sslCertificateFileLocationEnabled</key>
            <integer>0</integer>
            <key>sslKeyFileLocation</key>
            <string></string>
            <key>sslKeyFileLocationEnabled</key>
            <integer>0</integer>
            <key>type</key>
            <string>SPTCPIPConnection</string>
            <key>useSSL</key>
            <integer>0</integer>
            <key>user</key>
            <string>%s</string>
        </dict>
    </dict>
    <key>encrypted</key>
    <false/>
    <key>format</key>
    <string>connection</string>
    <key>queryFavorites</key>
    <array/>
    <key>queryHistory</key>
    <array/>
    <key>rdbms_type</key>
    <string>mysql</string>
    <key>rdbms_version</key>
    <string>5.5.44</string>
    <key>version</key>
    <integer>1</integer>
</dict>
</plist>`

// DdevRouterTemplate is the template for the generic router container.
const DdevRouterTemplate = `version: '{{ .compose_version }}'
services:
  ddev-router:
    image: {{ .router_image }}:{{ .router_tag }}
    container_name: ddev-router
    ports:{{ $dockerIP := .dockerIP }}{{ if not .router_bind_all_interfaces }}{{ range $port := .ports }}
    - "{{ $dockerIP }}:{{ $port }}:{{ $port }}"{{ end }}{{ else }}{{ range $port := .ports }}
    - "{{ $port }}:{{ $port }}"{{ end }}{{ end }}
    volumes:
      - /var/run/docker.sock:/tmp/docker.sock:ro
      - ddev-global-cache:/mnt/ddev-global-cache:rw
      {{ if .letsencrypt }}
      - ddev-router-letsencrypt:/etc/letsencrypt:rw
      {{ end }}
    environment:
      - DISABLE_HTTP2={{ .disable_http2 }}
{{ if .letsencrypt }}
      - LETSENCRYPT_EMAIL={{ .letsencrypt_email }}
      - USE_LETSENCRYPT={{ .letsencrypt }}
{{ end }}
    restart: "{{ if .AutoRestartContainers }}always{{ else }}no{{ end }}"
    healthcheck:
      interval: 1s
      retries: 120
      start_period: 120s
      timeout: 120s

networks:
  default:
    name: ddev_default
    external: true
volumes:
  ddev-global-cache:
    name: ddev-global-cache
{{ if .letsencrypt }}
  ddev-router-letsencrypt:
    name: ddev-router-letsencrypt
{{ end }}
`

const DdevSSHAuthTemplate = `version: '{{ .compose_version }}'

volumes:
  dot_ssh:
    name: "ddev-${DDEV_SITENAME}_dot_ssh"
  socket_dir:
    name: ddev-ssh-agent_socket_dir

services:
  ddev-ssh-agent:
    container_name: ddev-ssh-agent
    hostname: ddev-ssh-agent
    build:
      context: '{{ .BuildContext }}'
      args:
        BASE_IMAGE: {{ .ssh_auth_image }}:{{ .ssh_auth_tag }}
        username: '{{ .Username }}'
        uid: '{{ .UID }}'
        gid: '{{ .GID }}'
    image: {{ .ssh_auth_image }}:{{ .ssh_auth_tag }}-built
    restart: "{{ if .AutoRestartContainers }}always{{ else }}no{{ end }}"
    user: "$DDEV_UID:$DDEV_GID"
    volumes:
      - "dot_ssh:/tmp/.ssh"
      - "socket_dir:/tmp/.ssh-agent"
    environment:
      - SSH_AUTH_SOCK=/tmp/.ssh-agent/socket
    healthcheck:
      interval: 1s
      retries: 2
      start_period: 10s
      timeout: 62s
networks:
  default:
    name: ddev_default
    external: true
`
