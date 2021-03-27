# automutek8s -- GKE hosted automuteus

## Getting Started

### Synopsis

1. Install [mage](https://github.com/magefile/mage)
2. [Install gcloud](https://cloud.google.com/sdk/docs/install);
   [Homebrew](https://brew.sh/) and [its linux variant](https://docs.brew.sh/Homebrew-on-Linux),
   or [Chocolatey](https://chocolatey.org/) is easy.
3. Make your [Discord application](https://discord.com/developers/applications) and bot.
4. Provision your cluster (see below)
5. Setup secrets (see below)
6. Deploy automuteus (see below)
7. Invite the bot to your discord server

### Configure Discord Bot

After creation of Discord application, go to OAuth2 URL Generator to make a link to add bot.
Required scope is "bot".

Then we have to give permission to the bot.
According to invitation link described in [README.md](https://github.com/denverquane/automuteus),
AutoMuteUs bot requires permission in integer 267746384.
But it seems not to include "Manage Emojis" permission, which has to be enabled.

We will have to enable permissions listed below:

* Manage Channels
* Change Nicknames
* Manage Nicknames
* View Channels
* Manage Emojis

* Send Messages
* Send TTS Messages
* Manage Messages
* Embed Links
* Read Message History
* Use External Emojis
* Add Reactions

* Connect
* Speak
* Mute Members
* Deafen Members
* Move Members
* Use Voice Activity

So we will get URL like this:

```
https://discord.com/api/oauth2/authorize?client_id=CLIENTID&permissions=1341488208&scope=bot
```

### Cluster Provisioning

automutek8s has terraform configuration to provision GKE cluster.
To make terraform operational, we have to setup backend.
And `terraform` mage task will do it.


```
$ gcloud config configurations create automutek8s
$ gcloud config set core/project $YOUR_PROJECT_ID
$ gcloud auth application-default login
$ mage terraform
$ ls auto.tf.json
auto.tf.json
$ terraform init
```

`mage terraform` will check the project name to
make terraform manages the resource correctly.
Then just do `terraform apply` as usual.

```
$ terraform apply
```

GKE cluster creation will take about 5 minutes.

### Managing Secrets

[AutoMuteUs](https://github.com/denverquane/automuteus) has several secrets to
configure.
In automutek8s we will manage the secrets with mage task and [Cloud Secret Manager](https://cloud.google.com/secret-manager).

To see what secrets we have to set, invoke `secrets:list` task.

```
$ mage secrets:list
discordbot DISCORD_BOT_TOKEN = (none)
postgres POSTGRES_PASSWORD = (none)
postgres POSTGRES_USER = (none)
```

We can set secret by `secrets:set`.

```
$ mage secrets:set postgres POSTGRES_USER dbuser
```

```
$ vi pgpass.txt
$ mage secrets:set postgres POSTGRES_PASSWORD $PWD/pgpass.txt
```

Now the secrets are set.

```
$ mage secrets:list
discordbot DISCORD_BOT_TOKEN = (none)
postgres POSTGRES_PASSWORD = <filtered>
postgres POSTGRES_USER = <filtered>
```

### Deploying automuteus

```
$ mage gke:getCredentials
$ mage kustomization
$ kustomize build | kubectl apply -f -
```

## Chance of Improvement

* Stop using public GKE endpoint for security. `kubectl` invocation should go to Cloud Build.
* Enable TLS (GAE flexible as a reverse proxy will work well)
* Provide means of remote or automatic cluster shutdown to make saving money easier.
* Tweak Kubernetes CPU / memory requests

## Questions?

### Hey, you're using k8s but it seems unscallable!

We have single PostgreSQL sever and single Redis server in Kubernates manifests.
Yes, it cannot be scalled out by adding pods.

But in our personal use case, it's important to stop all pods by `kubectl delete all --all` or
entire cluster by `terraform destroy` to save money.
We don't play Among Us for thousands of years at once.
We just need it in a couple of hours for a day.

If you have neccessity to make clusters can be scalled out while keeping higher availability,
you can swith PostgreSQL and Redis to managed goodies like
Cloud SQL and Cloud Memorystore.

## Where is TLS access?

On GCP, the cheapest way to have a TLS frontend,
it will be having reverse proxy on GAE standard environment. 
Thanks to [net/http/httputil](https://golang.org/pkg/net/http/httputil/) in golang standard library.

GAE standard is great! Because: 

* It provides us domain name (like foo.us.r.appspot.com) for free.
* It provides TLS by default.
* It can be scaled in to zero while not being used.
* It can be scaled out automatically.

Meanwhile, we have a problem here: [Galactus](https://github.com/automuteus/galactus) needs WebSockets access,
and standard environment does not support WebSockets. Geez.

First option will be GAE flexible environment.

* Yes, it provides us domain name (like foo.us.r.appspot.com) as well.
* Yes, it provides TLS too.
* We have to disable auto scalling to allow us to stop instance.

Second option will be self-hosting; in other words,
buying a domain, reserving a public IP address, creating TLS certificates,
and host a name server. 
