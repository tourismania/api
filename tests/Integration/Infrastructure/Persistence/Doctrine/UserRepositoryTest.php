<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Tests\Integration\Infrastructure\Persistence\Doctrine;

use App\Domain\Entity\User;
use App\Domain\Repository\UserRepositoryInterface;
use App\Infrastructure\Persistence\Doctrine\User\UserRepository;
use Symfony\Bundle\FrameworkBundle\Test\KernelTestCase;

class UserRepositoryTest extends KernelTestCase
{

    public function testStore(): void
    {
        self::bootKernel();

        $repository = $this->createMock(UserRepository::class);
        $repository->expects($this->once())
            ->method('store')
            ->willReturn(null);

        $result = $repository->store(new User('a', 'b', 'c', 'd'), 'e');
        $this->assertEquals(null, $result);
    }
}
