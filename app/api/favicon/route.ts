import { NextResponse } from 'next/server';
import { getFaviconData } from '@/lib/favicon';

export async function GET(request: Request) {
  const { searchParams } = new URL(request.url);
  const url = searchParams.get('url');

  if (!url) {
    return NextResponse.json({ error: 'Missing url query parameter.' }, { status: 400 });
  }

  try {
    const result = await getFaviconData(url);
    return NextResponse.json(result);
  } catch (cause) {
    const message = cause instanceof Error ? cause.message : 'Something went wrong.';
    return NextResponse.json({ error: message }, { status: 400 });
  }
}
