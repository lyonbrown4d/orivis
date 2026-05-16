import { FormEvent, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Activity } from 'lucide-react';
import { PreferenceControls } from '@/components/PreferenceControls';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { errorMessage, fetchAuthSession, login } from '@/lib/api';

export default function LoginPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const redirect = params.get('redirect') || '/';
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const sessionQuery = useQuery({ queryKey: ['auth-session'], queryFn: fetchAuthSession, retry: false });
  const loginMutation = useMutation({
    mutationFn: () => login(username, password),
    onSuccess: () => navigate(redirect, { replace: true })
  });

  if (sessionQuery.data?.authenticated) {
    return <Navigate to={redirect} replace />;
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    loginMutation.mutate();
  };

  return (
    <main className="grid min-h-screen place-items-center bg-[radial-gradient(circle_at_20%_20%,_rgba(20,184,166,0.28),_transparent_28%),linear-gradient(145deg,_#f8fafc,_#e0f2fe_45%,_#f1f5f9)] p-5 text-slate-950 dark:bg-[radial-gradient(circle_at_20%_20%,_rgba(45,212,191,0.18),_transparent_28%),linear-gradient(145deg,_#020617,_#0f172a_55%,_#111827)] dark:text-white">
      <div className="w-full max-w-md space-y-5">
        <div className="flex justify-center">
          <PreferenceControls compact />
        </div>
        <Card className="border-white/70 bg-white/80 shadow-2xl shadow-teal-900/10 backdrop-blur-xl dark:border-slate-800 dark:bg-slate-950/75">
          <CardHeader className="text-center">
            <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-3xl bg-slate-950 text-white shadow-xl shadow-teal-500/20 dark:bg-white dark:text-slate-950">
              <Activity className="h-6 w-6" />
            </div>
            <CardTitle className="text-3xl font-black tracking-tight">{t('loginTitle')}</CardTitle>
            <p className="text-sm text-slate-500 dark:text-slate-400">{t('loginSubtitle')}</p>
          </CardHeader>
          <CardContent>
            <form className="space-y-4" onSubmit={submit}>
              <label className="block space-y-2">
                <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">{t('username')}</span>
                <Input value={username} onChange={(event) => setUsername(event.target.value)} autoComplete="username" className="h-11 bg-white text-slate-950 dark:bg-slate-900 dark:text-white" />
              </label>
              <label className="block space-y-2">
                <span className="text-sm font-semibold text-slate-700 dark:text-slate-200">{t('password')}</span>
                <Input
                  value={password}
                  type="password"
                  onChange={(event) => setPassword(event.target.value)}
                  autoComplete="current-password"
                  className="h-11 bg-white text-slate-950 dark:bg-slate-900 dark:text-white"
                />
              </label>
              {loginMutation.isError && (
                <p className="rounded-2xl bg-rose-50 px-4 py-3 text-sm font-medium text-rose-700 dark:bg-rose-950 dark:text-rose-300">
                  {errorMessage(loginMutation.error, t('invalidLogin'))}
                </p>
              )}
              <Button type="submit" className="h-11 w-full rounded-full text-sm font-bold" disabled={loginMutation.isPending || !username || !password}>
                {loginMutation.isPending ? t('refreshing') : t('signIn')}
              </Button>
            </form>
          </CardContent>
        </Card>
        <p className="text-center text-xs text-slate-500 dark:text-slate-400">{t('hideCredentials')}</p>
      </div>
    </main>
  );
}
