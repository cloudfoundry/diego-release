# Pushing a CF Application to the Diego Runtime

1. Create and target a CF org and space:

  ```bash
  cf api --skip-ssl-validation api.bosh-lite.com
  cf auth admin admin
  cf create-org diego
  cf target -o diego
  cf create-space diego
  cf target -s diego
  ```

1. Change into your application directory and push your application without starting it:

  ```bash
  cd <app-directory>
  cf push my-app --no-start
  ```

1. [Enable Diego](https://github.com/cloudfoundry/diego-design-notes/blob/master/migrating-to-diego.md#targeting-diego) for your application.

1. Start your application:

  ```bash
  cf start my-app
  ```
