# sslreminder

Check expiration date of SSL certificates periodically, then remind you via email.

## Set up

This is primary intended to work on Heroku.
Thanks to Heroku, sslreminder can work as free of charge.

By following commands, sslreminder checks 3 hosts 
`1.example.com`, `2.example.com` and `3.example.com` every day.
It sends reminder to `alice@example.com` and `bob@example.com` if
any of certificates expire in 30 days.

    git clone git@github.com:tkawachi/sslreminder.git
    heroku create -b https://github.com/kr/heroku-buildpack-go.git
    heroku config:set HOSTS=1.example.com,2.example.com,3.example.com \
      EMAILS=alice@example.com,bob@example.com
    heroku addons:add sendgrid:starter
    heroku ps:scale clock=1

You can ensure that it works by looking logs.

    heroku logs

If you want to be reminded earlier, set `THRESHOLD_DAYS`.

    # Remind me 60 days before the expiration
    heroku config:set THRESHOLD_DAYS=60

From address of email is defaulted to the first address of `EMAILS`.
You can change it by setting `FROM`.

    heroku config:set FROM=taro@example.com
