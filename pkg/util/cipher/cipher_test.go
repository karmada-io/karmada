/*
 * @Version : 1.0
 * @Author  : wangxiaokang
 * @Email   : xiaokang.w@gmicloud.ai
 * @Date    : 2025/05/15
 * @Desc    : 加密解密测试
 */

package cipher

import (
	"testing"
)

var (
	kb1 = ``
	kb2 = ``
	kb3 = `#!/bin/bash

DRY_RUN=false

log() {
  local level="${1:-INFO}"
  local message="$2"
  local timestamp=$(date "+%Y-%m-%d %H:%M:%S")
  
  # set color based on log level
  local color=""
  local reset="\033[0m"
  
  case "$level" in
    INFO)  color="\033[0;32m" ;; # green
    WARN)  color="\033[0;33m" ;; # yellow
    ERROR) color="\033[0;31m" ;; # red
    *)     color="\033[0m"    ;; # default
  esac
  
  # output to console (with color) and log file (without color)
  if [ "$DRY_RUN" = false ]; then
    echo -e "${color}[${timestamp}] [${level}] ${message}${reset}" >> {{.LOG_PATH}}
  else
    echo -e "${message}"
  fi
}

init() {
  mkdir -p {{.MOUNT_POINT}} || true

  CURL_PATH=$(which curl 2>/dev/null || echo "")
  if [ -z "$CURL_PATH" ]; then
    apt update
    apt install -y curl
  else
    log INFO "curl already installed at $CURL_PATH"
  fi

  FUSE_PATH=$(which fusermount 2>/dev/null || echo "")
  if [ -z "$FUSE_PATH" ]; then
    apt update
    apt install -y fuse
  else
    log INFO "fuse already installed at $FUSE_PATH"
  fi

  # check gcloud cli
  GCLOUD_PATH=$(which gcloud 2>/dev/null || echo "")
  if [ -z "$GCLOUD_PATH" ]; then
    log INFO "gcloud cli is not installed, installing it..."
    # add google cloud sdk source
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list

    # install apt-transport-https ca-certificates gnupg
    apt-get install -y apt-transport-https ca-certificates gnupg
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -

    # update and install cloud sdk
    apt-get update
    apt-get install -y google-cloud-sdk
  else
    log INFO "gcloud cli already installed at $GCLOUD_PATH"
  fi

  # create gcloud config dir
  if [ ! -d ~/.config/gcloud ]; then
    mkdir -p ~/.config/gcloud
  fi

  if [ ! -f ~/.config/gcloud/application_default_credentials.json ]; then
    # create application_default_credentials.json
    cat <<EOF > ~/.config/gcloud/application_default_credentials.json
{{.GCP_APPLICATION_DEFAULT_CREDENTIALS}}
EOF
  fi
}

mount() {
  init
  # install JuiceFS client
  if [ ! -f "{{.JUICEFS_PATH}}" ]; then
    log INFO "JuiceFS client not found, downloading..."
    curl -L {{.JUICEFS_CONSOLE_HOST}}/onpremise/juicefs -o {{.JUICEFS_PATH}} && chmod +x {{.JUICEFS_PATH}}
  else
    # get current installed JuiceFS version
    CURRENT_VERSION=$({{.JUICEFS_PATH}} version 2>/dev/null | awk '{print $3}')
    # compare version
    if [ "$CURRENT_VERSION" != "{{.JUICEFS_VERSION}}" ]; then
      log INFO "JuiceFS version mismatch (current: $CURRENT_VERSION, required: {{.JUICEFS_VERSION}}), downloading new version..."
      curl -L {{.JUICEFS_CONSOLE_HOST}}/onpremise/juicefs -o {{.JUICEFS_PATH}} && chmod +x {{.JUICEFS_PATH}}
      {{.JUICEFS_PATH}} version -u
    fi
  fi

  {{.JUICEFS_PATH}} mount --cache-dir {{.JUICEFS_CACHE_DIR}} --token {{.JUICEFS_TOKEN}} {{.JUICEFS_NAME}} {{.MOUNT_POINT}} {{.MOUNT_OPTIONS}}

}

umount() {
  {{.JUICEFS_PATH}} umount {{.MOUNT_POINT}}
  rm -rf {{.MOUNT_POINT}}
  if [ -f "{{.STORAGE_PATH}}" ]; then
    rm -f "{{.STORAGE_PATH}}"
    log INFO "storage path removed: {{.STORAGE_PATH}}"
  fi
  log INFO "{{.JUICEFS_NAME}} umount done"
}

dry_run() {
  echo "dry run"
  DRY_RUN=true
  init
  mount
  umount
  DRY_RUN=false
}

main(){
  case $1 in
    mount) mount ;;
    umount) umount ;;
    dry-run) dry_run ;;
    *) log ERROR "Usage: $0 {mount|umount|dry-run}" ;;
  esac
}

main "$@"`
)

func TestEncrypt(t *testing.T) {
	cases := []struct {
		Name string
		text []byte
	}{
		{"a", []byte(kb1)},
		{"b", []byte(kb2)},
		{"c", []byte(kb3)},
	}
	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := EncryptCompact(c.text, "uOvKLmVfztaXGpNYd4Z0I1SiT7MweJhl"); err != nil {
				t.Fatalf("encrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("%s encrypt text is { %s }", c.Name, ans)
			}
		})
	}
}

func TestDecrypt(t *testing.T) {
	cases := []struct {
		Name string
		text string
	}{
		{"a", `KAl-BcH6vzo1WQ4WogEQPcKvEnRgolYNJRNwuudvModdTpaSj338T4QtdElnHUPiY9X8tT5VG__7EJv9x9VO8D75SjrEAecFaWtWEdAEwW0297Cl6quYS47dUVfCcDO0Xsmt8j6im5Ac8u4rfYIo-22hRmc0TEOcwemsQh26PMUxsHoiDkedAETUwzNLVWzMDPE8vJX0WRycyM-vAm3euKo9Zq2Ijk73mLSrFvWrSpsUYyxKadsOPQM7XlM491qLO95xGwupiaIdje2th7MhrasoxO9jE31QV4o0gzfKCyWRDlny4CSyKVaQStLK9YrML7BXNcadlUuAM97eaGxx6m7IWAxvCGXltmoTWPTIpYFNwxgEyZqtYyUWzB4KShNX3TKL7mu2VPUw9giyVO2Ys9xPj5KfCOlYtP4DdkH1FnVOY-Mb94X3khlqF2fVOl-S7cQwgsRsa_QsMz1gd4iu1EQ-4CIqslEaU_AmiZi1IPWjJQRg3rL_OXcMZ5RGLry3`},
		{"b", "HlZO9LEf_QCcd8gMTKTjNndMz9Cw7d51lDke14sbrWZvNM-LfpRjZpakfv53xLve9Z0nFaz12V1GDSsNqnO11LaVBZP4gMyvfyCJLFWNBIf7ODFM2MN9jziUmUS78F7UMq2YmaXbRPAgLMZibJdjapNzh6oneJOD8Vy8NvXqD9PgT1hNNJZsvcl66e2kRwy_G1d5JyiXoSazgzyViYsm9wqXIth1_WNilaRxLlHCmzXkMZ8mmSJzoKivoCOMeq1kTMuQebZjTYk0W7LGG8lB4uDcuMxveNUjo3Q5xlOdjdMouiQ8jz5tUn9OwP8RbEI4AzTHMiCYocK19xHIYU4AXBpoMH6J4HO1WJdF9mrH0imrpdBxsRmvBe1c0JbSmq2G42eltjES9KbemJJS0umC3M1b_271_xwPPWKRhTc0Mwa8c4eYsttFTz8XKTZVr3MxhAN1A-88QJgHLDUECdUe3-kLoz23oXAZqZWRpKpUFfwMPOPvcsnJ15ZXOMnYsQpW42tCD1B3PW4NkfclDu984U-wy6p6LA5r6Ae-eBKaNMtsqI6hl9nWlAGaL41U6L7VNjuDpAm3j1tsIsgl192S210cxMe-EEXOb7m76UTN4Ck4dOJkQu0lALYQVs60j8j_iCd2_R418vuIrRNhX4iBeuTTUEZp9CDwCKtijrjXn0F87lE1rmU3pdBNODxgtwxO1uU3Yj5A1gu0d-dz60zimIwdnr-f3mBOF3E-IFm1yiDeQiFG1Vg-sDzWB319bUm6izEwXTmx3A_ZvL7O1Mh8EO08HLlWf_81IoMulY5vUygk0dXqAcDXAnn1ElhW-kLE8xP5sDm39ZtSn8Lv9yEZotXCXVfgwrSPs-JTqL3hTsse3Q6jbKipEobGumojnpSejp2RMfr2ZgMugpuesuUuVdhhb61Dm8AAob2z-jsWAXy3PPbf97PmWVoBCW4MgAsNWKX93O4RhPJObJtylkfLnDnkANbH6gRZm2UgxTHpdNd29NaEkW693Bn28ifAm58O-04Em2134wixUDNPcEzjCHbVm7N7yG-MAlNn3zhiSrrqHB738gXfHCYGdWSg0yYUoXSEYQHsvzlRf_MXbvviXmHME8nV5N_xAkuxwPojxliIdi4gRPVR9bZoqK1_IP-OYe716EO3kSBuJRjkStFDAR-JhW2vxAytvh4Ov-iQQjrGefrk1NAybJLGvEH_FAF4BLTtZX24Eg1NVgtNWYPmEna42JBeoxKR-ZGaMZA3Pm3DmSMpbQpBXydLzbFnqNpLJewamUeKyBCr0WCb9GnyHybbzS2tLLH0e8hVjpyPaeumtmTi_c3_BrAgA6y4gjezU3FSrtljG5ZZWaX0pViiNLyMNdTqI27SXip2FuF2WhMsURju2RLNx6tpxKPy6CI8SNGzdAHwZ0Y8YTIIxCRjMWAQDuerhe-1f_6vgEGoZRNxmTlpMZAHi9oDgMQdddc3CPsitBksQywqSX2wadN7RONIaeldTLR8yX72tEpGhSqOHMVjnF1IfbtRKJBAuWGmylIyc06SNAojm-0jILM1eiC_kvCnfznbihOaLceQNnCp9Qq93vOAfG3GL1YDkK-ZI9DmOl-ODTLtKnniU-VgpfLLRUf4yWUoSQ7JsALd5padQRAapZZvMFigvnobOATYf90cE54DV2N2cSh-EB1jUs5pWc5taIG3oAS0GkgVNDAFRkEWbR5GZDyaxaISbHjvLPQrxwgvKRUH1hRepbYaiLbE4PO2cJ2coskPQl3-KuPHBRK0GNHAxVPhMdX2es-A2Bk0zbo4EYjFIR31Ndu9mztIgXadXS2pyEpAsTiwAw5RRMBCnZyIR29rcxLchf-R1bYQlW-qUeA1P1DMaHYenD8cMrMRoho4gv0tBgveHj8K5kFj7saRny2mr9aAWmuKQRQSpnhqwC3glnQK8YylUxAswKpk8S9wU29gf0SPSMabgRnYCM36fpUTHqgu0n_QHBD874hTkIpv3yB9WIfq7FT-9wLxos9ANMNt6CZ_1qOAGOmI_P58LCAwFem0Vj2HzaTfxsdDFq1Sz3OhGuQKctCD6cSkDV2ped7EDH_merxgjH0gWd_Zi9vdCkw1Be752tlFt5DKFrn6Y_voMnWrOh_mqBSgCxVSMagw8Kc-YJ0UgSWTk3h7uW4rch1K6Sg7Y7mPiWJA1CxaHd_MwapaWLdm0d0Pm0zsMzaZLUdEf_iH0Md5SxpImWOUaPWp_r4kW3s3yOwvUey0uu_4Q7LvZdElgKpBl-a7TUwkowZfO1YlO97wrYnP0nhMGky6_4GPTMhkICuDVrNwZE-Ysxvfjj60v6AcQdaLZwbqNiCT_ri_P7JfXzxGqGvhC64Wl-3Uf6iWZPOnEI5BBF2RUwE6S30p5-9Itz0fF2MGW_9OCdaAcS8QEio25zwiffhq8_s_IODjGZY9ltrAVBuBR2RXKB1ZdBm9egmVnOztWAIoOPvpnUsKeR2u_lZ95rQsap9LBd3aS7kX54bhY86117EHUEP4XbF_h6MPFq36xhIrXnNh0H2UJNCVCJDABZrZtbn-pjEvbpPuZ456SZ8pTmqlwN-whGxwYuPAVvSYgbwPnRx05V9v0tusohCerN03aZ0LiewhT1MfgRllXVDejB9VFICNh0xNxx_CmQofyTYBTRF9ukOibxXwBsWdc-2Ghn2f5M0OcM3QAn5QRMVLNxFa1Zg-pqSd-LOBWd5b5hY6GWyZZgWyM-avS11pTYc8ogldHFE2AwqseMZEnGAgZY1OA_2xBBQ0TkADveNdREiNhCV8vFlk176tR49It3RmoGGZNkNQSEAboJtKEiYwUA7gFdXnPXeTxcexeLmox5xlam7Iwd1aObUErAsz3jV_Lfqm-TSdeCg6Sy0Sb6YozqMp-JQs8_bj7x6odDq-CFJsNV7W2Tc2UC0SOM1HJlpsRGlZsgk3PuwlYk-OL43tyMhEp0zOuuljDqyndQOWM503kwrKgtect_29Z-Kyh1IoiPxNf9jxvFYy7QoPdrgCgwzd0msugGQpvMZbuJ8jn1_6ounPWSwahv86b0ee97r2oNCJnPXJozd1Yj6j_bXtAS-93rCLHeY4-tWPLkarko6MuisnhnDukwJ5eE8icE6LHP_0SL_LALSo-YCZA4_WCOMDe4VYWtwTVEdHoU5y_BHybjhTSgAMPCuM0EBb01l7QPOyrMkcXaU2LhG0m-WLueyTU2Lq5YOAIYa-bFsAnE4Pc5H7JjRX77aZIg6FtZCSZHXPd0oZVqUO26u-XuybVJJui3hec-lSFXAjSnl5eTvGomNn0ByOjUVwlVbsLUJ5xXL7EpCbif10ec4PvnCOCuZELBtxH6NLsjbcjoUNuVnXTcaFnoc56TvwPT6lJiVsrYYkRe8EBx5JffMTYWR13offuvHsduhgJd1uZg0XhB4FaUZ71I8kYIc8OuivFzsuWQhpW67CyLYZrHnVbbz0RZxUlmI13qA4DbSc5CafFvm26EVSjFeyTvl-fkSBMnRPh3Tcw6uNhR_TPrbT6U5iNay39BI2Ua9D8eJXSfD-w4YpJfIDMu43kg0QWVpEv9DTdB_t2Jzx45VFCEI1L6AOgtQ2GUxi-prbYz2dbePRL-AaJ0BCzMCGzbLpyXJ_yGFXt_xK5Rm7XBCREkIHpo0Cu-Q3t7t7qGn90fkEiMpNZG5OEqsntc6_1E_CBuTJR8aNxIdEqjBoMIS_HzLAC6Dn0BQbFsfS2RVjlW5BWiJEquyMe-Rq7OiUqNOc5y21kwDQeuVKVGmOl2DZrUCaGKA4GwqT3tTNGVfnVQFUpMWq_c05wDQirFsK2rXhoAT1sfNhHc2OsgLuSIxr7i24N20gxjheFIx5-Nro6SgmO0R32vyEvPQ0PpuMrjtfoeFABYe71RN4M-uuS7EzUc5ueQXY1ayIYA"},
		{"c", "ho1s5qTr+11w9t/9W/d6yg=="},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if ans, err := DecryptCompact(c.text, "uOvKLmVfztaXGpNYd4Z0I1SiT7MweJhl"); err != nil {
				t.Fatalf("decrypt text %s failed: %+v",
					c.text, err)
			} else {
				t.Logf("decrypt is %s", ans)
			}
		})
	}
}
