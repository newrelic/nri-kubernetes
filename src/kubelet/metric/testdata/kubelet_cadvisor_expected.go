package testdata

import (
	"github.com/newrelic/nri-kubernetes/src/definition"
)

// ExpectedCadvisorRawData ...
var ExpectedCadvisorRawData = definition.RawGroups{
	"container": {
		"kube-system_heapster-5mz5f_heapster": {
			"containerID":      "015ff1fea2583aba674c824c754de8c3a0ef52ee4bb82b9bbc523be8f346393c",
			"containerImageID": "k8s.gcr.io/heapster-amd64@sha256:da3288b0fe2312c621c2a6d08f24ccc56183156ec70767987501287db4927b9d",
		},
		"kube-system_influxdb-grafana-rsmwp_grafana": {
			"containerID":      "7f092105225a729f4917aa6950b5b90236c720fc411eee80ba9f7ca0f639525f",
			"containerImageID": "k8s.gcr.io/heapster-grafana-amd64@sha256:4a472eb4df03f4f557d80e7c6b903d9c8fe31493108b99fbd6da6540b5448d70",
		},
		"kube-system_influxdb-grafana-rsmwp_influxdb": {
			"containerID":      "fd0ca055e308e5d11b0c8fbf273b733d1166aa2823bf7fd724a6b70c72959774",
			"containerImageID": "k8s.gcr.io/heapster-influxdb-amd64@sha256:f433e331c1865ad87bc5387589965528b78cd6b1b2f61697e589584d690c1edd",
		},
		"kube-system_kube-addon-manager-minikube_kube-addon-manager": {
			"containerID":      "48b12201acc975f6ac563b3c4938e835a6bd161bfdfc0bb8594c144c8a422c99",
			"containerImageID": "sha256:d166ffa9201aa156eb76d3a221c3fdab07bb1a0b6407548b1b1f03dc111c0e39",
		},
		"kube-system_kube-dns-54cccfbdf8-dznm7_dnsmasq": {
			"containerID":      "81de1e9aba1c051a2f9780a5db594a899c9e4e76613d4c95da4561cc48e8658f",
			"containerImageID": "sha256:459944ce8cc4f08ebade5c05bb884e4da053d73e61ec6afe82a0b1687317254c",
		},
		"kube-system_kube-dns-54cccfbdf8-dznm7_kubedns": {
			"containerID":      "fa38c736dddefc876009abac2338fc936ebadc82e3708817d7f852d94084e655",
			"containerImageID": "sha256:512cd7425a731bee1f2a3e4c825fc1cfe516c8b7905874f24bee6da12801d663",
		},
		"kube-system_kube-dns-54cccfbdf8-dznm7_sidecar": {
			"containerID":      "8f2f88385d4754e0631974ea8faf242608840f4e5afee98e96618b8b235b7fde",
			"containerImageID": "sha256:fed89e8b4248a788655d528d96fe644aff012879c782784cd486ff6894ef89f6",
		},
		"kube-system_kube-state-metrics-57f4659995-6n2qq_addon-resizer": {
			"containerID":      "3328c17bfd22f1a82fcdf8707c2f8f040c462e548c24780079bba95d276d93e1",
			"containerImageID": "gcr.io/google_containers/addon-resizer@sha256:e77acf80697a70386c04ae3ab494a7b13917cb30de2326dcf1a10a5118eddabe",
		},
		"kube-system_kube-state-metrics-57f4659995-6n2qq_kube-state-metrics": {
			"containerID":      "c452821fcf6c5f594d4f98a1426e7a2c51febb65d5d50d92903f9dfb367bfba7",
			"containerImageID": "quay.io/coreos/kube-state-metrics@sha256:52a2c47355c873709bb4e37e990d417e9188c2a778a0c38ed4c09776ddc54efb",
		},
		"kube-system_kubernetes-dashboard-77d8b98585-mtjld_kubernetes-dashboard": {
			"containerID":      "413bbcacdd1ea51fd3471beac717af55cd771f08993ce7b5fe66803835dc8421",
			"containerImageID": "sha256:e94d2f21bc0c297cb74c1dfdd23e2eace013f532c60726601af67984d97f718a",
		},
		"kube-system_newrelic-infra-rz225_newrelic-infra": {
			"containerID":      "69d7203a8f2d2d027ffa51d61002eac63357f22a17403363ef79e66d1c3146b2",
			"containerImageID": "sha256:1a95d0df2997f93741fbe2a15d2c31a394e752fd942ec29bf16a44163342f6a1",
		},
		"kube-system_storage-provisioner_storage-provisioner": {
			"containerID":      "473c90e8ec9e958c38e191b31fa1df8e705f6bde444351cb6ea5cf2fdef43ba2",
			"containerImageID": "gcr.io/k8s-minikube/storage-provisioner@sha256:088daa9fcbccf04c3f415d77d5a6360d2803922190b675cb7fc88a9d2d91985a",
		},
	},
}
