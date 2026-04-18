<template>
  <el-dialog
    v-model:visible="dialogVisible"
    title="您访问的页面不存在"
    :close-on-click-modal="false"
    :show-close="false"
    :close-on-press-escape="false"
  >
    <el-button
      type="primary"
      @click="jump"
    >
      确定
    </el-button>
  </el-dialog>
</template>

<script>
export default {
  name: 'NotFound',
  data () {
    return {
      dialogVisible: true
    }
  },
  methods: {
    jump () {
      this.$router.push('/')
    }
  }
}
</script>
