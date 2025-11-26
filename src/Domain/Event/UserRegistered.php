<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Domain\Event;

use App\Infrastructure\BrokerMessages\Kafka\EncodeInterface;
use Symfony\Component\Messenger\Attribute\AsMessage;

#[AsMessage('kafka_async')]
readonly class UserRegistered implements EncodeInterface
{
    public function __construct(public int $id)
    {
    }

    public function getKey(): string
    {
        return (string) $this->id;
    }

    public function getEventCode(): string
    {
        return 'user_registered';
    }
}
