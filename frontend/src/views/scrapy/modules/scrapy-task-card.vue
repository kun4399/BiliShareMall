<script setup lang="ts">
import { computed } from 'vue';
import { NButton, NCard, NIcon, NTag, NText, NTime } from 'naive-ui';
import { Play, StopSharp } from '@vicons/ionicons5';
import type { dao } from '~/wailsjs/go/models';
import { describeTaskUiState, type TaskUiState } from '@/features/scrapy/task-state';

defineOptions({
  name: 'ScrapyTaskCard'
});

const props = defineProps<{
  task: dao.ScrapyItem;
  orderLabel: string;
  taskState: TaskUiState;
  isRunning: boolean;
}>();

const emit = defineEmits<{
  close: [];
  run: [];
  stop: [];
}>();

const statusMeta = computed(() => describeTaskUiState(props.taskState));

const metricItems = computed(() => [
  {
    label: '折扣',
    value: props.task.discountFilterLabel || '不限'
  },
  {
    label: '价格',
    value: props.task.priceFilterLabel || '不限'
  },
  {
    label: '爬取次数',
    value: String(props.task.nums || 0)
  },
  {
    label: '完成循环次数',
    value: String(props.task.increaseNumber || 0)
  }
]);
</script>

<template>
  <NCard class="scrapy-task-card" size="small" closable @close="emit('close')">
    <template #header>
      <div class="task-card-title">
        <span>{{ task.productName }}</span>
        <NText depth="3">{{ orderLabel }}</NText>
      </div>
    </template>

    <template #header-extra>
      <div class="task-card-extra">
        <NTime class="task-card-time" :time="new Date(task.createTime)" />
        <NButton
          v-if="!isRunning"
          class="task-card-action"
          strong
          ghost
          circle
          size="medium"
          @click="emit('run')"
        >
          <template #icon>
            <NIcon><Play /></NIcon>
          </template>
        </NButton>
        <NButton v-else class="task-card-action" strong ghost circle size="medium" @click="emit('stop')">
          <template #icon>
            <NIcon><StopSharp /></NIcon>
          </template>
        </NButton>
      </div>
    </template>

    <div class="task-card-body">
      <div class="task-status-row">
        <NTag class="task-status-tag" :type="statusMeta.tagType" round size="small">
          {{ statusMeta.tagLabel }}
        </NTag>
        <NText depth="3" class="task-status-text">{{ statusMeta.text }}</NText>
      </div>

      <div class="task-metrics-grid">
        <div v-for="item in metricItems" :key="item.label" class="task-metric-item">
          <NText depth="3" class="task-metric-label">{{ item.label }}</NText>
          <NText class="task-metric-value">{{ item.value }}</NText>
        </div>
      </div>
    </div>
  </NCard>
</template>

<style scoped>
.scrapy-task-card :deep(.n-card-header) {
  padding-bottom: 10px;
}

.task-card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.task-card-extra {
  display: flex;
  align-items: center;
  gap: 10px;
}

.task-card-time {
  color: gray;
  font-size: 12px;
}

.task-card-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-status-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  flex-wrap: wrap;
}

.task-status-tag {
  flex-shrink: 0;
}

.task-status-text {
  line-height: 1.5;
}

.task-metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 8px;
}

.task-metric-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--n-color-embedded);
}

.task-metric-label {
  font-size: 12px;
}

.task-metric-value {
  font-size: 16px;
  font-weight: 600;
  line-height: 1.3;
}

@media (max-width: 640px) {
  .task-card-extra {
    gap: 8px;
  }

  .task-card-time {
    display: none;
  }
}
</style>
