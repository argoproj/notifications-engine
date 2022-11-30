# Matrix

**NOTE:** native end-to-end encryption (e2ee) for Matrix notifications is not yet supported because CGO, which is needed to link to [libolm](https://gitlab.matrix.org/matrix-org/olm), is not supported by Argo.  Those who want end-to-end encryption support for their Argo notifications bot can setup [pantalaimon](https://github.com/matrix-org/pantalaimon).

To be able to send notifications via Matrix, do the following steps:

1. [Register a Matrix account](#register-a-matrix-account)
2. [Generate an access token and device ID for the account](#generate-an-access-token-and-device-id-for-the-account)
3. [Upload a profile picture (optional)](#upload-a-profile-picture-optional)
4. [Configure notifiers and subscription recipients](#configure-notifiers-and-subscription-recipients)

## Register a Matrix account

Registering a Matrix account can be done via a standard Matrix client like [Element](https://element.io) or many others listed at <https://matrix.org/clients>.

If your homeserver is a Synapse instance and you have access to the `registration_shared_secret`, which is only available to people with shell access to Synapse, you can register a new user with the [`/_synapse/admin/v1/register` endpoint](https://matrix-org.github.io/synapse/latest/admin_api/register_api.html).

## Generate an access token and device ID for the account

Before beginning, ensure you have `curl`, `jq`, and standard unix shell utilities installed.

Set the environment variables `USERID` and `PASSWORD` to your argo user's ID and password, respectively:

```sh
# your argo user's ID.  Of the form "@localpart:domain.tld"
export USERID="@argocd:example.org"
# set this to the password for your argo user.  If you need to use a different
# authentication method, the commands in this guide won't work
export PASSWORD="ch@ngeMe!"
```

Then, run the following commands:

```sh
export SERVER_NAME=$(printf "$USERID" | cut -d: -f2-)
export HOMESERVER_URL=$(curl -LSs https://${SERVER_NAME}/.well-known/matrix/client | jq -r '."m.homeserver"."base_url"')

RESP=`curl -d "{\"type\": \"m.login.password\", \"identifier\": {\"type\": \"m.id.user\", \"user\": \"$USERID\"}, \"password\": \"$PASSWORD\"}" \
    -X POST $HOMESERVER_URL/_matrix/client/v3/login`

export ACCESS_TOKEN=`printf "$RESP" | jq -r .access_token`
export DEVICEID=`printf "$RESP" | jq -r .device_id`

echo "Access Token: $ACCESS_TOKEN"
echo "Device ID: $DEVICEID"
```

You can now use the Access Token and Device ID printed in the last command as the respective parameters in the next section.

## Upload a profile picture (optional)

It is recommended, though not required, to give your argo user a profile picture, which you'll see next to all argocd Matrix notifications.

**NOTE**: this uses some of the environment variables set in the last section.

```sh
curl -LSs https://argocd-operator.readthedocs.io/en/stable/assets/logo.png > profile.png

RESP=`curl --data-binary @profile.png \
    -H 'Content-Type: image/png' \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    "$HOMESERVER_URL/_matrix/media/v3/upload?filename=profile.png"`

PROFILE_URI=`printf "$RESP" | jq -r .content_uri`

curl -X PUT -d "{\"avatar_url\": \"$PROFILE_URI\"}" \
    -H "Authorization: Bearer $ACCESS_TOKEN" $HOMESERVER_URL/_matrix/client/v3/profile/$USERID/avatar_url
```

## Configure notifiers and subscription recipients

The Matrix notification service requires specifying the following settings:

* `accessToken` - the access token retrieved after logging in.  This was displayed at the end of the [Generate an access token and device ID for the account](#generate-an-access-token-and-device-id-for-the-account) section
* `deviceID` - the device ID.  Retrieved alongside the access token at the end of the [Generate an access token and device ID for the account](#generate-an-access-token-and-device-id-for-the-account) section
* `homeserverURL` - optional, the homeserver base URL.  If unspecified, the base URL will be retrieved using the [well-known URI](https://spec.matrix.org/v1.3/client-server-api/#well-known-uri), if possible
* `userID` - the user ID.  Of the form `@localpart:server.tld`
