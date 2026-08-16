import { createApp } from 'vue';
import VueVirtualScroller from 'vue-virtual-scroller';
import App from './App.vue';
import { router } from './router';
import { useDb } from './db';

useDb();

const app = createApp(App);
app.use(router);
app.use(VueVirtualScroller);
app.mount('#app');