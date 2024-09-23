# Manual e2e testing

As of now, the [apptest-framework](https://github.com/giantswarm/apptest-framework) used for automated e2e testiong doesn't support MC-only apps. Hence the manual procedure described here in order to ensure that the app works as expected in a Giant Swarm environment.

## Procedure

Before proceeding to any kind of test, you'll first have to deploy your custom branch app's version into a testing installation. Don't forget to suspend flux reconciliation for this app during the whole testing process. See [here](https://intranet.giantswarm.io/docs/dev-and-releng/flux/suspending-flux/#how-to-be-more-granular--subtle-with-suspending-resources-and-why-be-careful-with-this) for details on how to evict an app from flux's reconciliation.

Then, run the `basic_test.sh` file which will check that everything is working as expected. Note that you have to specify the installation's name on which you want to execute the checks, so if you deployed your app on `grizzly` for example, run the following command from repo's root : `tests/manual_e2e/basic_test.sh grizzly`  

Once that's done, there are additional things you should do :

- Let it run for a bit, like 10min or more.
- Inspect the `TODO : find continuous-test dashboard` dasboard which will give information mimir's overall health. This is made possible through the use of the `mimir-continuous-test` component that is deployed by default in our mimir setup. For more information on it if you feel the need to tune it for your tests, head over to the [official documentation page](https://grafana.com/docs/mimir/latest/manage/tools/mimir-continuous-test/).
- Also inspect any other dashboard and make sure that you can see data from all WCs, including the one from the `basic_test.sh` file.
- If everything appears to be fine, then you can revert the flux's evicting procedure that you did and let it reconcile to its original version.

Congratulations, you have completed the manual e2e testing procedure ! Your PR is now ready to be merged.
