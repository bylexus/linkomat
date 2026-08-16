import { createApp } from 'vue';
import VueVirtualScroller from 'vue-virtual-scroller';
import App from './App.vue';
import { router } from './router';
import './state/auth';
import { initSync } from './sync/engine';

initSync();

const app = createApp(App);
app.use(router);
app.use(VueVirtualScroller);
app.mount('#app');