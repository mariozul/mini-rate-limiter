// CI pipeline for mini-rate-limiter. Mirrors the standard astronautsid Go
// service shape: SAST (Semgrep) → Build/Test → Security scan (Gosec) →
// SonarQube + Quality Gate. Runs on a Kubernetes agent with one container
// per tool so heavy images don't block the pipeline pod startup time.
//
// Deployment (CD) is intentionally out of scope here — image build/push
// and rollout are handled by the GitOps repo, not this Jenkinsfile. If you
// fork this scaffold and need an in-pipeline deploy stage, add it after
// "Quality Gate" and inject the target via credentials.

pipeline {
  agent {
    kubernetes {
      yaml '''
        apiVersion: v1
        kind: Pod
        spec:
          containers:
          - name: go
            image: golang:1.26
            command:
            - sleep
            args:
            - 999999
            tty: true
            resources:
              requests:
                memory: "1536Mi"
                cpu: "1000m"
              limits:
                cpu: "1000m"
            volumeMounts:
              - mountPath: /go/pkg/
                name: nfs-jenkins
              - mountPath: /root/.cache/
                name: nfs-jenkins
          - name: gosec
            image: securego/gosec:latest
            command:
            - sleep
            args:
            - 999999
            tty: true
            resources:
              limits: {}
              requests:
                memory: "100Mi"
                cpu: "100m"
          - name: semgrep
            image: returntocorp/semgrep:0.100.0
            command:
            - sleep
            args:
            - 999999
            tty: true
            resources:
              limits: {}
              requests:
                memory: "100Mi"
                cpu: "100m"
          - name: semgrep-jenkins
            image: asia-southeast2-docker.pkg.dev/dogwood-wharf-316804/base-image/astro-sast-semgrep-jenkins
            command:
            - sleep
            args:
            - 999999
            tty: true
            resources:
              limits: {}
              requests:
                memory: "100Mi"
                cpu: "100m"
          volumes:
            - name: nfs-jenkins
              persistentVolumeClaim:
                claimName: nfs-jenkins-storage-pvc
        '''
    }
  }
  options {
    disableConcurrentBuilds(abortPrevious: true)
  }
  stages {
    // Semgrep and Build run in parallel — Semgrep operates on source only
    // and has no dependency on the Go compile or test output, so wall-clock
    // time is bounded by max(semgrep, build+test) instead of their sum.
    stage("Static Analysis + Build") {
      parallel {
        stage("Semgrep Scan") {
          steps {
            container("semgrep") {
              sh 'semgrep ci --json --config=$SEMGREP_RULES_URI > gl-sast-report.json || true'
            }
            container("semgrep-jenkins") {
              withCredentials([string(credentialsId: 'semgrep-slack-webhook', variable: 'SEMGREP_SLACK_WEBHOOK')]) {
                sh 'cat ./gl-sast-report.json | /app/astro-sast-semgrep-jenkins -w $SEMGREP_SLACK_WEBHOOK -h $SEMGREP_API_URI -r $GIT_URL -b true'
              }
            }
          }
        }
        stage("Build and Test") {
          steps {
            checkout scm
            container('go') {
              withCredentials([usernamePassword(credentialsId: 'github-token', usernameVariable: 'USERNAME', passwordVariable: 'GITHUB_TOKEN')]) {
                sh 'git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"'
              }
              sh 'ln -sf /usr/share/zoneinfo/Asia/Jakarta /etc/localtime'
              sh 'go build -v -o grpc ./cmd/grpc'
              sh "go test -short -timeout=300s -covermode=atomic -coverprofile=cover.out -race -gcflags=all=-l ./..."
            }
          }
        }
      }
    }
    stage('Gosec Scan') {
      steps {
        container('gosec') {
          withCredentials([usernamePassword(credentialsId: 'github-token', usernameVariable: 'USERNAME', passwordVariable: 'GITHUB_TOKEN')]) {
            sh 'git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/astronautsid".insteadOf "https://github.com/astronautsid"'
            sh 'gosec -no-fail -fmt=sonarqube -out gosec-results.json ./...'
          }
        }
      }
    }
    stage("Sonar Scanner") {
      environment {
        scannerHome = tool 'sonar-scanner'
      }
      steps {
        container('jnlp') {
          withSonarQubeEnv('astro-sonar') {
            sh "${scannerHome}/bin/sonar-scanner -Dproject.settings=sonar.properties"
          }
        }
      }
    }
    stage("Quality Gate") {
      steps {
        script {
          timeout(time: 1, unit: 'MINUTES') {
            sleep(10)
            waitForQualityGate abortPipeline: true
          }
        }
      }
    }
  }
}
