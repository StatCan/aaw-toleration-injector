[(Français)](#injecteur-de-toleration-pour-ads-eaa)

# DAaaS AAW Toleration Injector

## Adding a node pool
When adding a new node pool type, the toleration injector should be extended to include tolerations for the new node pool. Once the taints have been defined for the nodes, they can be injected as tolerations to the appropriate pods. To allow specific namespaces to tolerate to special node pools, a [config map](https://github.com/StatCan/aaw-kubeflow-profiles#profiles-requiring-scheduling-to-special-node-pools) was created to map the node types to allowed namespaces.
## Development Workflow
1. Modify mutate.go to include the appropriate case (this should be refactored to a switch block)
2. Create a PR to main
3. Once merged, get docker image sha from github actions
4. Modify the reference in [aaw-argocd-manifests](https://github.com/StatCan/aaw-argocd-manifests/pull/154/files#diff-aa7775a5d7c9b88f528afe33704be4c4a75fdcc739e3a7a30b3aaffb5db3dafcL65)
then PR changes into the appropriate environment branch

## Testing
For testing, once the image is built at step 3, the following steps can be taken instead:
1. Modify the reference in a local branch of aaw-argocd-manifest
2. Disable auto-sync in argocd for the "statcan-system" app (notify your fellow devs :smiley:)
3. Deploy the manifest to aaw-dev using `kubectl apply -f` 
4. To revert, simply reference the previous image and reapply or enable auto-sync in argo

___
### How to Contribute

See [CONTRIBUTING.md](CONTRIBUTING.md)

### License

Unless otherwise noted, the source code of this project is covered under Crown Copyright, Government of Canada, and is distributed under the [MIT License](LICENSE).

The Canada wordmark and related graphics associated with this distribution are protected under trademark law and copyright law. No permission is granted to use them outside the parameters of the Government of Canada's corporate identity program. For more information, see [Federal identity requirements](https://www.canada.ca/en/treasury-board-secretariat/topics/government-communications/federal-identity-requirements.html).

______________________

## Injecteur de Toleration pour ADS EAA

### Comment contribuer

Voir [CONTRIBUTING.md](CONTRIBUTING.md)

### Licence

Sauf indication contraire, le code source de ce projet est protégé par le droit d'auteur de la Couronne du gouvernement du Canada et distribué sous la [licence MIT](LICENSE).

Le mot-symbole « Canada » et les éléments graphiques connexes liés à cette distribution sont protégés en vertu des lois portant sur les marques de commerce et le droit d'auteur. Aucune autorisation n'est accordée pour leur utilisation à l'extérieur des paramètres du programme de coordination de l'image de marque du gouvernement du Canada. Pour obtenir davantage de renseignements à ce sujet, veuillez consulter les [Exigences pour l'image de marque](https://www.canada.ca/fr/secretariat-conseil-tresor/sujets/communications-gouvernementales/exigences-image-marque.html).
