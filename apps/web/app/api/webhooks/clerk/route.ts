/* eslint-disable @typescript-eslint/no-unused-vars */
import { Webhook } from 'svix';
import { headers } from 'next/headers';
import { WebhookEvent } from '@clerk/nextjs/server';
import { PrismaClient } from '@prisma/client';
import type { Prisma } from '@prisma/client';
import { NextResponse } from 'next/server';

const prisma = new PrismaClient();

export async function POST(req: Request) {
  // You can find this in the Clerk Dashboard -> Webhooks -> choose the webhook
  const WEBHOOK_SECRET = process.env.CLERK_WEBHOOK_SECRET;

  if (!WEBHOOK_SECRET) {
    throw new Error('Please add CLERK_WEBHOOK_SECRET from Clerk Dashboard to .env or .env.local');
  }

  // Get the headers
  const headerPayload = await headers();
  const svix_id = headerPayload.get("svix-id");
  const svix_timestamp = headerPayload.get("svix-timestamp");
  const svix_signature = headerPayload.get("svix-signature");

  // If there are no headers, error out
  if (!svix_id || !svix_timestamp || !svix_signature) {
    return new Response('Error: No svix headers', { status: 400 });
  }

  // Get the body
  const payload = await req.json();
  const body = JSON.stringify(payload);

  // Create a new Svix instance with your secret.
  const wh = new Webhook(WEBHOOK_SECRET);
  let evt: WebhookEvent;

  // Verify the payload with the headers
  try {
    evt = wh.verify(body, {
      "svix-id": svix_id,
      "svix-timestamp": svix_timestamp,
      "svix-signature": svix_signature,
    }) as WebhookEvent;
  } catch (err) {
    console.error('Error verifying webhook:', err);
    return new Response('Error verifying webhook', { status: 400 });
  }

  // Get the ID and type
  const { id } = evt.data;
  const eventType = evt.type;

  console.log(`Webhook received with type: ${eventType}`);

  // --- YOUR SYNC LOGIC ---
  if (eventType === 'user.created') {
    const { id, email_addresses, first_name, last_name } = evt.data;
    const email = email_addresses[0]?.email_address;

    if (!id || !email) {
      return new Response('Error: Missing user ID or email', { status: 400 });
    }

    try {
      // Use a Prisma transaction to create the User, Org, and OrgMember
  await prisma.$transaction(async (tx: Prisma.TransactionClient) => {
        // Create the user
        const newUser = await tx.user.create({
          data: {
            id: id,
            email: email,
            name: `${first_name} ${last_name}`,
          },
        });

        // Create their personal organization
        const newOrg = await tx.organization.create({
          data: {
            name: `${first_name}'s Team`,
            plan: 'free',
          },
        });

        // Link the user to the org as its OWNER
        await tx.orgMember.create({
          data: {
            userId: newUser.id,
            orgId: newOrg.id,
            role: 'OWNER',
          },
        });
      });

      console.log(`Successfully synced new user: ${id}`);

    } catch (e) {
      console.error("Error during user sync transaction:", e);
      return new Response('Error during user sync', { status: 500 });
    }
  }

  if (eventType === 'user.updated') {
    // Handle user updates if needed (e.g., name, email changes)
  }

  if (eventType === 'user.deleted') {
    // Handle user deletion (e.g., cascade delete in Prisma schema handles this)
  }

  return new Response('', { status: 200 });
}