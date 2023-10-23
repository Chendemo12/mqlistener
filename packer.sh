#!/bin/bash

# 读取control文件内容
content=$(cat ./project/DEBIAN/control)

# 提取Package标签内容
PACKAGE=$(echo "$content" | grep -Po '(?<=Package:)\s*\S+' | tr -d '[:space:]')
# 提取Version标签内容
VERSION=$(echo "$content" | grep -Po '(?<=Version:)\s*\S+' | tr -d '[:space:]')
# 提取Architecture标签内容
ARCH=$(echo "$content" | grep -Po '(?<=Architecture:)\s*\S+' | tr -d '[:space:]')

APP_BIN_DIR="project/opt/apps/bin"
# 待删除的目录名数组
DIRTY_DIRECTORIES=("tmp" "bin")


# 删除目录
delete_directories() {
  # 接受待删除的目录名数组作为参数
  local dirs=("$@")

  # 遍历目录数组
  for dir in "${dirs[@]}"; do
    # 检查目录或文件是否存在
    if [[ -e "$dir" ]]; then
      # 删除目录或文件
      rm -rf "$dir"
    fi
  done
}

# 创建目录
create_directories() {
  local directories=("$@")  # 将输入的参数数组赋值给本地变量

  for dir in "${directories[@]}"; do
    if [ ! -d "$dir" ]; then  # 如果目录不存在
      mkdir "$dir"            # 创建目录
      chmod -R 755 "$dir"     # 设置目录权限为755
    else
      chmod -R 755 "$dir"
    fi
  done
}

# 检测当前运行目录下是否存在 build.sh 文件并编译文件
build_source() {
  mkdir -p ./tmp/$APP_BIN_DIR/
  if [ -f "build.sh" ]; then
    echo "正在运行 build.sh 编译脚本..."
    # 执行 build.sh 文件
    chmod +x ./build.sh
    ./build.sh

    # 检查 build.sh 是否返回错误
    if [ $? -ne 0 ]; then
      echo "执行 build.sh 时发生错误，程序终止"
      exit 1
    else
      echo "编译完成."
      move_bin "./bin" "./tmp/$APP_BIN_DIR"
    fi
  fi
}

# 将编译文件移动到目标目录
move_bin() {
  local bin_dir="$1"
  local apps_dir="$2"

  # 获取bin目录下最新的文件名
  # shellcheck disable=SC2012
  latest_file=$(ls -t "$bin_dir" | head -1)

  # 判断是否存在最新文件
  if [[ -n "$latest_file" ]]; then
    # 移动最新文件到apps目录
    mv "$bin_dir/$latest_file" "$apps_dir/$PACKAGE"
  else
    echo "未发现编译文件"
    exit 1
  fi
}

# dpkg 打包
make_package() {
  rsync -ah ./project ./tmp/ \
    --exclude=.idea --exclude=.git --exclude=logs \
    --exclude=vendor --exclude=venv

  # 确保拥有可执行权限
  chmod -R 775 ./tmp/project/DEBIAN/*
  chmod -R 775 ./tmp/$APP_BIN_DIR

  mkdir -p ./deb

  # 强制使用gzip打包deb
  # Ubuntu >= 21.04 Supported
  local p=deb/"${PACKAGE}"_"${VERSION}"_"${ARCH}".deb
  dpkg-deb -Zxz --build --root-owner-group tmp/project "$p"

  echo "deb output: $p"
}


# 修改版本号
echo -e "package main

const Version = \"v${VERSION}\"
" > ./source/version.go

# 设置GO环境变量
export GOOS=linux
export GOARCH=arm64


echo "**********************************************"
echo "*"
echo "* Package: '$PACKAGE'"
echo "* Version: '$VERSION'"
echo "* Architecture: '$ARCH'"
echo "*"
echo "**********************************************"
echo ""

delete_directories "${DIRTY_DIRECTORIES[@]}";
create_directories "${DIRTY_DIRECTORIES[@]}";

build_source;
make_package;
delete_directories "${DIRTY_DIRECTORIES[@]}";

exit 0
