def ws = "/data/jenkins/workspace/${JOB_NAME}-${BUILD_NUMBER}"
def quayImage = 'quay.io/newrelic/infrastructure-k8s-staging'
def quayE2eImage = 'quay.io/newrelic/infrastructure-k8s-e2e'
def integrationPath = '.'

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
    stage('Cancelling previously running builds') {
      steps {
        cancelPreviousBuilds()
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
                sh "docker build -t ${DOCKER_IMAGE_UNPRIVILEGED} --build-arg 'MODE=unprivileged' --label '${DOCKER_EXPIRES_LABEL}' ${integrationPath} && docker push ${DOCKER_IMAGE_UNPRIVILEGED}"
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
                branch 'main'
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

    stage('Privileged e2e tests') {
      parallel {
        stage('Privileged: 1.15.7')    { steps { runPrivilegedE2ETest('e2e-cluster-1-15-7-medium') } }
        stage('Privileged: 1.16.7')    { steps { runPrivilegedE2ETest('e2e-cluster-1-16-7') } }
        stage('Privileged: 1.17.9')    { steps { runPrivilegedE2ETest('e2e-cluster-1-17-9-medium') } }
      }
    }

    stage('Unprivileged e2e tests') {
      parallel {
        stage('Unprivileged: 1.15.7')  { steps { runUnprivilegedE2ETest('e2e-cluster-1-15-7-medium') } }
        stage('Unprivileged: 1.16.7')  { steps { runUnprivilegedE2ETest('e2e-cluster-1-16-7') } }
        stage('Unprivileged: 1.17.9')  { steps { runUnprivilegedE2ETest('e2e-cluster-1-17-9-medium') } }
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

def runPrivilegedE2ETest(clusterName) {
    // We lock the e2e build execution in order to run this step only once at a time.
    lock(resource: "k8s_cluster_${clusterName}") {
      build job: 'k8s-integration-e2e-concurrent', parameters: [
        string(name: 'CLUSTER_NAME', value: clusterName),
        string(name: 'INTEGRATION_IMAGE_TAG', value: "${DOCKER_TAG}"),
        string(name: 'RBAC', value: 'true'),
        string(name: 'UNPRIVILEGED', value: 'false'),
        string(name: 'VERBOSE', value: 'true'),
        string(name: 'E2E_DOCKER_IMAGE_TAG', value: "${DOCKER_TAG}")
      ]
  }
}

def runUnprivilegedE2ETest(clusterName) {
    // We lock the e2e build execution in order to run this step only once at a time.
    lock(resource: "k8s_cluster_${clusterName}") {
      build job: 'k8s-integration-e2e-concurrent', parameters: [
        string(name: 'CLUSTER_NAME', value: clusterName),
        string(name: 'INTEGRATION_IMAGE_TAG', value: "${DOCKER_TAG}_unprivileged"),
        string(name: 'RBAC', value: 'true'),
        string(name: 'UNPRIVILEGED', value: 'true'),
        string(name: 'VERBOSE', value: 'true'),
        string(name: 'E2E_DOCKER_IMAGE_TAG', value: "${DOCKER_TAG}")
      ]
    }
}

def cancelPreviousBuilds() {
   // Check for other instances of this particular build, cancel any that are older than the current one
   def jobName = env.JOB_NAME
   def currentBuildNumber = env.BUILD_NUMBER.toInteger()
   def currentJob = Jenkins.instance.getItemByFullName(jobName)

   // Loop through all instances of this particular job/branch
   for (def build : currentJob.builds) {
     if (build.isBuilding() && (build.number.toInteger() < currentBuildNumber)) {
       echo "Older build still queued. Sending kill signal to build number: ${build.number}"
       build.doStop()
     }
   }
}
