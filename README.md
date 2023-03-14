# Narcolepsy: a simple file-based CLI http client

Narcolepsy is a terminal HTTP API client, drawing inspiration from Ain, insomnia and VSCode's "rest client" extension.

- Organize your API's using files and folders
- Standard easy to read syntax: it's simply the HTTP standard spec
- support variables in request definitions which can be defined through a JSON config file
- multi-env support: define multiple environments in the config file
- simple and basic: it's basically VSCode's rest-client extension in a standalone CLI tool

## Installation

Currently, this requires that you have `go` installed:

```
go install github.com/naethiel/narcolepsy@latest
```

## Usage

```
NAME:
   Narcolepsy HTTP rest client - Send HTTP requests using .http or .rest files

USAGE:
   narc [options] [path/to/file]

AUTHOR:
   Nicolas Missika <n.missika@outlook.com>

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --verbose, -v                    Set log level to 'DEBUG' (default: false)
   --file value, -f value           Specify path to http request file
   --environment value, -e value    set the working environment (default: "default")
   --configuration value, -c value  path to the configuration file (default: "narcolepsy.json")
   --request value, -r value        name (without "###") of the request to run in the given file
   --help, -h                       show help

```

### Basic usage

In editor, type an HTTP request as simple as below:

```http
https://example.com/comments/1
```

You can follow the standard [RFC 2616](http://www.w3.org/Protocols/rfc2616/rfc2616-sec5.html) and define request method, headers, and body

```http

POST https://example.com/comments HTTP/1.1
content-type: application/json

{
    "name": "sample",
    "time": "Wed, 21 Oct 2015 18:27:50 GMT"
}

```

Then simply run `narc path/to/file.http`

**Note**
- defining protocol is optional and will default to `HTTP/1.1`
- defining method is optional and will default to `GET`
- having a single http request defined in a file will automatically send it upon invocation (see below for multiple requests per file)

### Multiple requests per file

You can define multiple requests in a single file and select which one to send. Separate requests with a line starting with 3 consecutive `#` chars. You can also use this separator line to define a name for your request.

```
### get comments

GET https://example.com/comments/1 HTTP/1.1

### get topics

GET https://example.com/topics/1 HTTP/1.1

### post comment

POST https://example.com/comments HTTP/1.1
content-type: application/json

{
    "name": "sample",
    "time": "Wed, 21 Oct 2015 18:27:50 GMT"
}

```

When using multiple requests in a single file, use the `--request` (or `-r`) flag to specify the name of the request you want to run if you want to skip the interactive selector entirely (useful when scripting/piping)

## Configuration

Narcolepsy configuration is lightweight: just create a configuration JSON file to define your environments and variables.

Example json file:

```json
{
  "environments": {
    "default": {
      "token": "###DEFAULT_TOKEN###",
    },
    "dev": {
      "token": "###DEV_TOKEN###",
      "version": "v2",
      "debug": "true"
    },
    "prod": {
      "version": "v1",
      "token": "###PROD_TOKEN###"
    }
  }
}

```

When running, Narcolepsy will try to find a `narcolepsy.json` file in the current directory and use it's values. You can specify a config file to use through the `--config` (`-c`) flag.

If you do not specify an environment to select, narcolepsy will fallback to the `"default"` env.

Once you have defined variables in a config file, you can use them as placeholders in your HTTP requests:

```
### get comments

GET https://example.com/{{version}}/comments
Authorization: {{token}}

```

### Example with config file and selected env:

- Config file: `config.json`

```json
{
  "environments": {
    "default": {
      "token": "###DEFAULT_TOKEN###",
    },
    "dev": {
      "token": "###DEV_TOKEN###",
      "version": "v2",
      "debug": "true"
    },
    "prod": {
      "version": "v1",
      "token": "###PROD_TOKEN###"
    }
  }
}
```

- Usage

```
narc requests.http --config=./config.json --env=dev
```

## Debugging

In case something is not behaving properly, you can add `--verbose` (`-v`) and narcolepsy will try to provide additional logging information to stdout
