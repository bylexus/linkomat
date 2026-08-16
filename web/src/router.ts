import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import HomeView from './views/HomeView.vue';
import EditView from './views/EditView.vue';

const routes: RouteRecordRaw[] = [
  { path: '/', component: HomeView },
  { path: '/entries/:id', component: EditView },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});