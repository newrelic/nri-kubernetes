def ws = "/data/jenkins/workspace/${JOB_NAME}-${BUILD_NUMBER}"
def quayImage = 'quay.io/newrelic/infrastructure-k8s-staging'
def quayE2eImage = 'quay.io/newrelic/infrastructure-k8s-e2e'
def integrationPath = '.'
def kubernetesTestCluster1_11 = 'fsi-jenkins-111-rbac'

pipeline {
  agent {
    node {
      label 'fsi-build-tests'
      customWorkspace "${ws}/go/src/github.com/newrelic/nri-kubernetes"
    }
  }
  options {
    buildDiscarder(logRotator(numToKeepStr: '15'))
    ansiColor('xterm')
  }

  environment {
    GOPATH = "${ws}/go"
    PATH = "${GOPATH}/bin:${PATH}"
    DOCKER_IMAGE = imageName(quayImage, branchName())
    DOCKER_IMAGE_UNPRIVILEGED = "${DOCKER_IMAGE}_unprivileged"
    DOCKER_TAG = tagName(branchName())
    DOCKER_EXPIRES_LABEL = "quay.expires-after=1w"
    E2E_DOCKER_IMAGE = "${quayE2eImage}:${DOCKER_TAG}"
    E2E_UNPRIVILEGED_DOCKER_IMAGE = "${quayE2eImage}:${DOCKER_TAG}_unprivileged"
  }

  stages {
    stage('Dependencies') {
      steps {
        withCredentials([string(credentialsId: 'KOPS_AWS_ACCESS_KEY_ID', variable: 'AWS_ACCESS_KEY_ID'), string(credentialsId: 'KOPS_AWS_SECRET_ACCESS_KEY', variable: 'AWS_SECRET_ACCESS_KEY')]) {
          sh 'aws s3 sync s3://nr-vendor-cache-fsi/vendor ./vendor --quiet'
        }
        sh 'make deps'
      }
    }
    stage('CI') {
      parallel {
        stage('Linting and Validation') {
          steps {
            sh 'make lint'
            sh 'make license-check'
          }
        }
        stage('Unit Tests') {
          steps {
            sh 'make test'
          }
        }
      }
    }
    stage('Building and pushing docker images') {
      parallel {
        stage('Integration') {
          steps {
            sh 'make compile'
            script {
              docker.withRegistry('https://quay.io/v2/', 'quay_fsi_robot') {
                // There is a known issue with Docker plugin and docker multi-stages. See: https://issues.jenkins-ci.org/browse/JENKINS-44609
                sh "docker build -t ${DOCKER_IMAGE} --label '${DOCKER_EXPIRES_LABEL}' ${integrationPath} && docker push ${DOCKER_IMAGE}"
                sh "docker build -t ${DOCKER_IMAGE_UNPRIVILEGED} --build-arg 'MODE=unprivileged' --build-arg 'IMAGE_TAG=1.3.5' --label '${DOCKER_EXPIRES_LABEL}' ${integrationPath} && docker push ${DOCKER_IMAGE_UNPRIVILEGED}"
              }
            }
          }
        }
        stage('e2e') {
          stages {
            stage('compile') {
              steps {
                dir(integrationPath) {
                  sh 'make e2e-compile-only'
                }
              }
            }
            stage('push') {
               steps {
                script {
                  docker.withRegistry('https://quay.io/v2/', 'quay_fsi_robot') {
                    docker.build("${E2E_DOCKER_IMAGE}", "-f ${integrationPath}/Dockerfile-e2e --label 'quay.expires-after=1w' ${integrationPath}").push()
                  }
                }
              }
            }
            stage('push latest') {
              when {
                branch 'master'
              }
              steps {
                script {
                  docker.withRegistry('https://quay.io/v2/', 'quay_fsi_robot') {
                    docker.build("${quayE2eImage}:latest", "-f ${integrationPath}/Dockerfile-e2e ${integrationPath}").push()
                  }
                }
              }
            }
          }
        }
      }
    }
    stage('Running e2e tests') {
      parallel {
        stage('privileged version') {
          steps {
            // We lock the e2e build execution in order to run this step only once at a time.
            lock(resource: "k8s_cluster_${kubernetesTestCluster1_11}", inversePrecedence: true) {
              build job: 'k8s-integration-e2e', parameters: [
                string(name: 'CLUSTER_NAME', value: kubernetesTestCluster1_11),
                string(name: 'INTEGRATION_IMAGE_TAG', value: "${DOCKER_TAG}"),
                string(name: 'RBAC', value: 'true'),
                string(name: 'UNPRIVILEGED', value: 'false'),
                string(name: 'VERBOSE', value: 'true'),
                string(name: 'E2E_DOCKER_IMAGE_TAG', value: "${DOCKER_TAG}")
              ]
            }
          }
        }
        stage('unprivileged version') {
          steps {
            // We lock the e2e build execution in order to run this step only once at a time.
            lock(resource: "k8s_cluster_${kubernetesTestCluster1_11}", inversePrecedence: true) {
              build job: 'k8s-integration-e2e', parameters: [
                string(name: 'CLUSTER_NAME', value: kubernetesTestCluster1_11),
                string(name: 'INTEGRATION_IMAGE_TAG', value: "${DOCKER_TAG}_unprivileged"),
                string(name: 'RBAC', value: 'true'),
                string(name: 'UNPRIVILEGED', value: 'true'),
                string(name: 'VERBOSE', value: 'true'),
                string(name: 'E2E_DOCKER_IMAGE_TAG', value: "${DOCKER_TAG}")
              ]
            }
          }
        }
      }
    }
  }
}

def branchName() {
  // CHANGE_BRANCH is map to the branch name when triggered from a Pull Request. Otherwise it does not exist.
  return env.CHANGE_BRANCH ?: env.BRANCH_NAME
}

def imageName(image, branch) {
  return image + ":" + tagName(branch)
}

def tagName(branch) {
  return branch.replace("/", "_")
}
