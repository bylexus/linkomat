import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import HomeView from './views/HomeView.vue';
import EditView from './views/EditView.vue';
import LoginView from './views/LoginView.vue';
import AboutView from './views/AboutView.vue';
import FriendsView from './views/FriendsView.vue';

const routes: RouteRecordRaw[] = [
  { path: '/login', component: LoginView },
  { path: '/about', component: AboutView },
  { path: '/', component: HomeView },
  { path: '/entries/:id', component: EditView },
  { path: '/friends', component: FriendsView },
];

export const router = createRouter({
  history: createWebHistory(),
  routes,
});
